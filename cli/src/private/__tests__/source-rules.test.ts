import { describe, expect, test } from "bun:test";
import * as fs from "node:fs";
import * as path from "node:path";

const PRIVATE_SOURCE_DIRECTORY = path.resolve(import.meta.dir, "..");

function privateSourceFiles(): string[] {
  const sourceFiles: string[] = [];
  const pendingDirectories = [PRIVATE_SOURCE_DIRECTORY];
  for (
    let visitCount = 0;
    pendingDirectories.length > 0 && visitCount < 64;
    visitCount++
  ) {
    const directory = pendingDirectories.pop()!;
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const entryPath = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        if (entry.name !== "__tests__") pendingDirectories.push(entryPath);
      } else if (entry.name.endsWith(".ts")) {
        sourceFiles.push(entryPath);
      }
    }
  }
  return sourceFiles;
}

/** Index after the first "{" that ends a line outside parentheses and angle brackets. */
function functionBodyStart(
  sourceText: string,
  declarationIndex: number
): number | undefined {
  let parenthesisDepth = 0;
  let angleDepth = 0;
  for (let index = declarationIndex; index < sourceText.length; index++) {
    const character = sourceText[index];
    if (character === "(") parenthesisDepth++;
    if (character === ")") parenthesisDepth--;
    if (character === "<") angleDepth++;
    if (character === ">" && sourceText[index - 1] !== "=") angleDepth--;
    if (character === ";" && parenthesisDepth === 0) return undefined;
    if (
      character === "{" &&
      parenthesisDepth === 0 &&
      angleDepth === 0 &&
      /^\s*\n/.test(sourceText.slice(index + 1, index + 3))
    ) {
      return index + 1;
    }
  }
  return undefined;
}

describe("cli/src/private source rules", () => {
  test("has source files", () => {
    expect(privateSourceFiles().length).toBeGreaterThan(5);
  });

  test("no package-qualified id literal (64 hex next to ':')", () => {
    const offenders = privateSourceFiles().filter((file) =>
      /[0-9a-f]{64}:|:[0-9a-f]{64}\b/.test(fs.readFileSync(file, "utf8"))
    );
    expect(offenders).toEqual([]);
  });

  test("no function calls itself by name", () => {
    const offenders: string[] = [];
    for (const file of privateSourceFiles()) {
      const sourceText = fs.readFileSync(file, "utf8");
      const declarations = [
        ...sourceText.matchAll(
          /^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)|^\s+(?:(?:private|public|protected|static|async)\s+)+(\w+)\s*\(/gm
        ),
      ];
      for (const declaration of declarations) {
        const functionName = declaration[1] ?? declaration[2];
        const bodyStart = functionBodyStart(sourceText, declaration.index!);
        if (bodyStart === undefined) continue;
        let depth = 1;
        let bodyEnd = bodyStart;
        for (; bodyEnd < sourceText.length && depth > 0; bodyEnd++) {
          if (sourceText[bodyEnd] === "{") depth++;
          if (sourceText[bodyEnd] === "}") depth--;
        }
        const body = sourceText.slice(bodyStart, bodyEnd);
        if (
          new RegExp(
            `(?<![\\w.])${functionName}\\s*\\(|this\\.${functionName}\\s*\\(`
          ).test(body)
        ) {
          offenders.push(`${path.basename(file)}:${functionName}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});

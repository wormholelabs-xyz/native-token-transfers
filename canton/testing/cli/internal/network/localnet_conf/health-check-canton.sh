#!/bin/bash
# Wormhole replacement for docker/canton/health-check.sh: the original three profile-gated
# probes, plus unconditional probes of the three new participants' gRPC health ports (they
# are not profile-gated -- see localnet.go's comment on APP_USER_PROFILE, they are always
# mounted/included once the canton container starts under any profile).
set -eou pipefail

if [ "$APP_USER_PROFILE" = "on" ]; then
  echo "Checking 2${CANTON_GRPC_HEALTHCHECK_PORT_SUFFIX}"
  grpc-health-probe -addr="localhost:2${CANTON_GRPC_HEALTHCHECK_PORT_SUFFIX}"
fi
if [ "$APP_PROVIDER_PROFILE" = "on" ]; then
  echo "Checking 3${CANTON_GRPC_HEALTHCHECK_PORT_SUFFIX}"
  grpc-health-probe -addr="localhost:3${CANTON_GRPC_HEALTHCHECK_PORT_SUFFIX}"
fi
if [ "$SV_PROFILE" = "on" ]; then
  echo "Checking 4${CANTON_GRPC_HEALTHCHECK_PORT_SUFFIX}"
  grpc-health-probe -addr="localhost:4${CANTON_GRPC_HEALTHCHECK_PORT_SUFFIX}"
fi

echo "Checking 5961 (bob)"
grpc-health-probe -addr="localhost:5961"
echo "Checking 6961 (guardian-governance)"
grpc-health-probe -addr="localhost:6961"
echo "Checking 7961 (guardian-observer)"
grpc-health-probe -addr="localhost:7961"

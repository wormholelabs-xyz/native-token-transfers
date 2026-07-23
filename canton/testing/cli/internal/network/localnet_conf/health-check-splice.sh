#!/bin/bash
# Wormhole replacement for docker/splice/health-check.sh: the original three profile-gated
# probes, plus unconditional probes of the three new validator apps' readyz endpoints.
set -eou pipefail

if [ "$APP_USER_PROFILE" = "on" ]; then
  wget --no-verbose --tries=1 --spider "http://localhost:2${VALIDATOR_ADMIN_API_PORT_SUFFIX}/api/validator/readyz"
fi
if [ "$APP_PROVIDER_PROFILE" = "on" ]; then
  wget --no-verbose --tries=1 --spider "http://localhost:3${VALIDATOR_ADMIN_API_PORT_SUFFIX}/api/validator/readyz"
fi
if [ "$SV_PROFILE" = "on" ]; then
  wget --no-verbose --tries=1 --spider "http://localhost:4${VALIDATOR_ADMIN_API_PORT_SUFFIX}/api/validator/readyz"
  wget --no-verbose --tries=1 --spider http://localhost:5012/api/scan/readyz
  wget --no-verbose --tries=1 --spider http://localhost:5014/api/sv/readyz
fi

wget --no-verbose --tries=1 --spider http://localhost:5903/api/validator/readyz
wget --no-verbose --tries=1 --spider http://localhost:6903/api/validator/readyz
wget --no-verbose --tries=1 --spider http://localhost:7903/api/validator/readyz

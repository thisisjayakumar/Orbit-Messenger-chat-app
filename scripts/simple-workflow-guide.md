# 1. Start everything (builds, starts, health checks)
./scripts/start-all-services.sh

# 2. Test everything is working
./scripts/test-services.sh

# 3. Stop everything
./scripts/stop-all-services.sh

# 4. Generate proto files (only when developing)
./scripts/generate-proto.sh
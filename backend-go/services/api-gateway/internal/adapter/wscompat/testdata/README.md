# testdata

`channel_inventory.json` is the sorted list of every WS channel the gateway registers (name and kind).
The frontend contract test (TASK-FE-E2E-001) reads it, so a channel the UI calls must exist here.

Regenerate after an intended change:

    UPDATE_GOLDEN=1 go test ./internal/adapter/wscompat -run TestChannelInventoryMatchesGolden

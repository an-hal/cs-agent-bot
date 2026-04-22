package cron

import (
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/conditiondsl"
)

// masterDataRecord wraps *entity.MasterData to satisfy conditiondsl.Record.
type masterDataRecord struct {
	*entity.MasterData
}

// wrapRecord creates a conditiondsl.Record from a MasterData entity.
func wrapRecord(md *entity.MasterData) conditiondsl.Record {
	return &masterDataRecord{MasterData: md}
}

// Ensure interface satisfaction at compile time.
var _ conditiondsl.Record = (*masterDataRecord)(nil)

package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OnchainDepositScanState struct {
	ent.Schema
}

func (OnchainDepositScanState) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "onchain_deposit_scan_states"},
	}
}

func (OnchainDepositScanState) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (OnchainDepositScanState) Fields() []ent.Field {
	return []ent.Field{
		field.String("chain").MaxLen(32).NotEmpty(),
		field.Int64("last_scanned_block").Default(0),
	}
}

func (OnchainDepositScanState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("chain").Unique(),
	}
}

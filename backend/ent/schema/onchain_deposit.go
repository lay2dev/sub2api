package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OnchainDeposit struct {
	ent.Schema
}

func (OnchainDeposit) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "onchain_deposits"},
	}
}

func (OnchainDeposit) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (OnchainDeposit) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("chain").MaxLen(32).NotEmpty(),
		field.String("token_symbol").MaxLen(16).NotEmpty(),
		field.String("token_contract").MaxLen(42).NotEmpty(),
		field.String("tx_hash").MaxLen(66).NotEmpty(),
		field.Int64("log_index"),
		field.Int64("block_number"),
		field.String("block_hash").MaxLen(66).NotEmpty(),
		field.String("from_address").MaxLen(42).NotEmpty(),
		field.String("to_address").MaxLen(42).NotEmpty(),
		field.String("amount_raw").MaxLen(80).NotEmpty(),
		field.Float("amount_credit").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("status").MaxLen(32).Default("detected"),
		field.Time("credited_at").Optional().Nillable(),
		field.String("error_message").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional().Nillable(),
	}
}

func (OnchainDeposit) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("chain", "tx_hash", "log_index").Unique(),
		index.Fields("chain", "status"),
		index.Fields("user_id"),
	}
}

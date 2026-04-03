package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RedeemCodeUsage holds the schema definition for redeem code usage rows.
type RedeemCodeUsage struct {
	ent.Schema
}

func (RedeemCodeUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_code_usages"},
	}
}

func (RedeemCodeUsage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("redeem_code_id"),
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Time("used_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RedeemCodeUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("redeem_code_id"),
		index.Fields("user_id"),
		index.Fields("api_key_id"),
		index.Fields("redeem_code_id", "user_id").Unique(),
	}
}

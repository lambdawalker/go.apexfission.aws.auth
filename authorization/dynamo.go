package authorization

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type GrantDB interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// DynamoGrants reads current server-owned environment grants consistently. Frontend
// page visibility, user claims and requested environment never create grants.
type DynamoGrants struct {
	DB    GrantDB
	Table string
}

func (d DynamoGrants) Authorize(ctx context.Context, p Principal, env, permission string) error {
	if p.Subject == "" || !ValidPermission(permission) {
		return ErrDenied
	}
	out, e := d.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(d.Table), ConsistentRead: aws.Bool(true), Key: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "GRANT#" + env}, "sk": &types.AttributeValueMemberS{Value: p.Subject}}})
	if e != nil {
		return ErrDenied
	}
	active, ok := out.Item["active"].(*types.AttributeValueMemberBOOL)
	if !ok || !active.Value {
		return ErrDenied
	}
	scopes, ok := out.Item["permissions"].(*types.AttributeValueMemberSS)
	if !ok {
		return ErrDenied
	}
	for _, s := range scopes.Value {
		if s == permission {
			return nil
		}
	}
	return ErrDenied
}

// GrantTable identifies the table for an atomic publication-time grant check.
func (d DynamoGrants) GrantTable() string { return d.Table }

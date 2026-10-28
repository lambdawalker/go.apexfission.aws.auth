package config

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"testing"
)

type db struct {
	tx *dynamodb.TransactWriteItemsInput
}

func (d *db) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{"active": &types.AttributeValueMemberBOOL{Value: true}, "permissions": &types.AttributeValueMemberSS{Value: []string{"sales:config.write"}}}}, nil
}
func (d *db) Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	return nil, nil
}
func (d *db) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	d.tx = in
	return &dynamodb.TransactWriteItemsOutput{}, nil
}
func TestPublicationChecksGrantInSameTransaction(t *testing.T) {
	d := &db{}
	s := Service{Store: Dynamo{DB: d, Table: "config"}, Authorize: authorization.DynamoGrants{DB: d, Table: "config"}, Schemas: map[string]Schema{"sales": {Version: 1, Validate: func(json.RawMessage) error { return nil }}}}
	_, e := s.Publish(context.Background(), authorization.Principal{Subject: "admin"}, "dev", "sales", 0, 1, json.RawMessage(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	if len(d.tx.TransactItems) != 3 || d.tx.TransactItems[2].ConditionCheck == nil {
		t.Fatal("grant not checked atomically")
	}
}

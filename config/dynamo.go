package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strconv"
)

type DynamoAPI interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}
type Dynamo struct {
	DB    DynamoAPI
	Table string
}

func attr(s string) types.AttributeValue { return &types.AttributeValueMemberS{Value: s} }
func partition(e, n string) string       { return "ENV#" + e + "#MODULE#" + n }
func itemKey(e, n, sort string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": attr(partition(e, n)), "sk": attr(sort)}
}
func decodeItem(m map[string]types.AttributeValue) (Revision, error) {
	v, ok := m["document"].(*types.AttributeValueMemberS)
	if !ok {
		return Revision{}, ErrNotFound
	}
	var r Revision
	e := json.Unmarshal([]byte(v.Value), &r)
	return r, e
}
func (d Dynamo) Get(ctx context.Context, e, n string) (Revision, error) {
	o, err := d.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(d.Table), Key: itemKey(e, n, "CURRENT"), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return Revision{}, err
	}
	return decodeItem(o.Item)
}
func (d Dynamo) Publish(ctx context.Context, expected int64, r Revision) error {
	return d.publish(ctx, expected, r, "", "")
}
func (d Dynamo) PublishGuarded(ctx context.Context, expected int64, r Revision, grantTable, permission string) error {
	return d.publish(ctx, expected, r, grantTable, permission)
}
func (d Dynamo) publish(ctx context.Context, expected int64, r Revision, grantTable, permission string) error {
	if expected < 0 || r.Revision != expected+1 {
		return ErrConflict
	}
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	makeItem := func(sk string) map[string]types.AttributeValue {
		m := itemKey(r.Environment, r.Namespace, sk)
		m["document"] = attr(string(b))
		m["revision"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(r.Revision, 10)}
		return m
	}
	condition := "revision = :expected"
	values := map[string]types.AttributeValue{":expected": &types.AttributeValueMemberN{Value: strconv.FormatInt(expected, 10)}}
	if expected == 0 {
		condition = "attribute_not_exists(pk)"
		values = nil
	}
	items := []types.TransactWriteItem{{Put: &types.Put{TableName: aws.String(d.Table), Item: makeItem("CURRENT"), ConditionExpression: aws.String(condition), ExpressionAttributeValues: values}}, {Put: &types.Put{TableName: aws.String(d.Table), Item: makeItem(fmt.Sprintf("REV#%020d", r.Revision)), ConditionExpression: aws.String("attribute_not_exists(pk)")}}}
	if grantTable != "" {
		items = append(items, types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: aws.String(grantTable), Key: map[string]types.AttributeValue{"pk": attr("GRANT#" + r.Environment), "sk": attr(r.Actor)}, ConditionExpression: aws.String("active = :active AND contains(permissions, :permission)"), ExpressionAttributeValues: map[string]types.AttributeValue{":active": &types.AttributeValueMemberBOOL{Value: true}, ":permission": attr(permission)}}})
	}
	_, e = d.DB.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: items})
	var canceled *types.TransactionCanceledException
	if errors.As(e, &canceled) {
		for _, r := range canceled.CancellationReasons {
			if aws.ToString(r.Code) == "ConditionalCheckFailed" {
				return ErrConflict
			}
		}
	}
	return e
}
func (d Dynamo) History(ctx context.Context, e, n string) ([]Revision, error) {
	out := []Revision{}
	o, err := d.DB.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(d.Table), KeyConditionExpression: aws.String("pk = :pk AND begins_with(sk, :rev)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": attr(partition(e, n)), ":rev": attr("REV#")}, ConsistentRead: aws.Bool(true), ScanIndexForward: aws.Bool(false), Limit: aws.Int32(100)})
	if err != nil {
		return nil, err
	}
	for _, m := range o.Items {
		r, e := decodeItem(m)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, nil
}

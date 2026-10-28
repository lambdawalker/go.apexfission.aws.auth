package sales

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strconv"
)

type Dynamo struct {
	DB    *dynamodb.Client
	Table string
}

func (d Dynamo) List(ctx context.Context, subject string) ([]Order, error) {
	out, e := d.DB.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(d.Table), KeyConditionExpression: aws.String("owner = :owner"), ExpressionAttributeValues: map[string]types.AttributeValue{":owner": &types.AttributeValueMemberS{Value: subject}}, Limit: aws.Int32(100)})
	if e != nil {
		return nil, e
	}
	rows := []Order{}
	for _, v := range out.Items {
		id, ok := v["id"].(*types.AttributeValueMemberS)
		if !ok {
			continue
		}
		amount, ok := v["amount"].(*types.AttributeValueMemberN)
		if !ok {
			continue
		}
		n, e := strconv.ParseInt(amount.Value, 10, 64)
		if e != nil {
			return nil, e
		}
		rows = append(rows, Order{id.Value, n})
	}
	return rows, nil
}
func (d Dynamo) Put(ctx context.Context, subject string, o Order) error {
	_, e := d.DB.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(d.Table), Item: map[string]types.AttributeValue{"owner": &types.AttributeValueMemberS{Value: subject}, "id": &types.AttributeValueMemberS{Value: o.ID}, "amount": &types.AttributeValueMemberN{Value: strconv.FormatInt(o.Amount, 10)}}, ConditionExpression: aws.String("attribute_not_exists(owner)")})
	return e
}

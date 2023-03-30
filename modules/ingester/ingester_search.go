package ingester

import (
	"context"

	"github.com/intergral/deep/pkg/deeppb"
	"github.com/pkg/errors"
	"github.com/weaveworks/common/user"
)

func (i *Ingester) SearchRecent(ctx context.Context, req *deeppb.SearchRequest) (*deeppb.SearchResponse, error) {
	instanceID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(instanceID)
	if !ok || inst == nil {
		return &deeppb.SearchResponse{}, nil
	}

	res, err := inst.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (i *Ingester) SearchTags(ctx context.Context, req *deeppb.SearchTagsRequest) (*deeppb.SearchTagsResponse, error) {
	instanceID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(instanceID)
	if !ok || inst == nil {
		return &deeppb.SearchTagsResponse{}, nil
	}

	res, err := inst.SearchTags(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (i *Ingester) SearchTagValues(ctx context.Context, req *deeppb.SearchTagValuesRequest) (*deeppb.SearchTagValuesResponse, error) {
	instanceID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(instanceID)
	if !ok || inst == nil {
		return &deeppb.SearchTagValuesResponse{}, nil
	}

	res, err := inst.SearchTagValues(ctx, req.TagName)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (i *Ingester) SearchTagValuesV2(ctx context.Context, req *deeppb.SearchTagValuesRequest) (*deeppb.SearchTagValuesV2Response, error) {
	instanceID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(instanceID)
	if !ok || inst == nil {
		return &deeppb.SearchTagValuesV2Response{}, nil
	}

	res, err := inst.SearchTagValuesV2(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SearchBlock only exists here to fulfill the protobuf interface. The ingester will never support
// backend search
func (i *Ingester) SearchBlock(context.Context, *deeppb.SearchBlockRequest) (*deeppb.SearchResponse, error) {
	return nil, errors.New("not implemented")
}

/*
 * Copyright (C) 2023  Intergral GmbH
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package ingester

import (
	"context"

	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/util"

	"github.com/pkg/errors"
)

func (i *Ingester) SearchRecent(ctx context.Context, req *deeppb.SearchRequest) (*deeppb.SearchResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(tenantID)
	if !ok || inst == nil {
		return &deeppb.SearchResponse{}, nil
	}

	res, err := inst.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (i *Ingester) SearchTags(ctx context.Context, _ *deeppb.SearchTagsRequest) (*deeppb.SearchTagsResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(tenantID)
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
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(tenantID)
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
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}
	inst, ok := i.getInstanceByID(tenantID)
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

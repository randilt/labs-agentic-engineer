// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package onboarding

import (
	"context"
	"errors"

	"github.com/wso2/aep/aep-api/internal/gen"
	"github.com/wso2/aep/aep-api/internal/platform/apierr"
	"github.com/wso2/aep/aep-api/internal/platform/tenant"
)

// Handler serves POST /projects/{projectName}/onboarding.
type Handler struct{ svc AnalysisStarter }

// AnalysisStarter is the slice's service port. *Service satisfies it.
type AnalysisStarter interface {
	StartAnalysis(ctx context.Context, orgID, projectID, sourceRepoRef string) (string, error)
}

// New returns the slice handler.
func New(svc AnalysisStarter) *Handler { return &Handler{svc: svc} }

func (h *Handler) StartOnboardingAnalysis(ctx context.Context, request gen.StartOnboardingAnalysisRequestObject) (gen.StartOnboardingAnalysisResponseObject, error) {
	if h.svc == nil {
		return nil, apierr.ServiceUnavailable("onboarding not configured")
	}
	org := tenant.BoundOrgFromContext(ctx)
	if request.Body == nil || request.Body.SourceRepoRef == "" {
		return nil, apierr.BadRequest("sourceRepoRef is required")
	}
	executionID, err := h.svc.StartAnalysis(ctx, org, request.ProjectName, request.Body.SourceRepoRef)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			return nil, apierr.NotFound("project not found")
		case errors.Is(err, ErrAnalysisInFlight):
			return nil, apierr.Conflict("analysis already in flight for this project")
		case errors.Is(err, ErrInvalidSourceRef):
			return nil, apierr.BadRequest(err.Error())
		default:
			return nil, apierr.Internal("failed to start onboarding analysis")
		}
	}
	return gen.StartOnboardingAnalysis202JSONResponse(gen.OnboardingAnalysisResponse{ExecutionID: executionID}), nil
}

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

// Package onboard is the platform-only executor for import-as-is components.
// It mints aep:onboard milestone issues at plan time, vendors source code
// from the component's source pointer into the project repo via an
// auto-merging pull request, and tears down legacy components on cutover.
// Build and deploy follow the ordinary fanOutBuilds path after merge — there
// is no parallel EnsureComponent/TriggerBuild dispatch here.
package onboard

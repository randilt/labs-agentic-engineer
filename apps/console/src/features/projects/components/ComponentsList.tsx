/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState } from "react";
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Stack,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Boxes } from "@wso2/oxygen-ui-icons-react";
import { EmptyState } from "../../../components/EmptyState";
import { StatusChip } from "../../../components/StatusChip";
import type { components } from "../../../generated/aep-api";
import { useSession } from "../../../auth/SessionContext";
import { chatKeyFor, setPendingSeed } from "../../agent-chat/chatStore";
import { useDesignDependencies } from "../../spec/api/queries";
import { ComponentOpenApiDialog } from "./ComponentOpenApiDialog";

type Component = components["schemas"]["Component"];
type ComponentDependencies = components["schemas"]["ComponentDependencies"];

const isWebApp = (c: Component) => c.type === "web-application";

function modeChip(dep: ComponentDependencies | undefined) {
  switch (dep?.sourceMode) {
    case "importAsIs":
      return <StatusChip label="imported" tone="neutral" appearance="soft" />;
    case "modernize":
      return <StatusChip label="modernizing" tone="info" appearance="soft" />;
    default:
      return null;
  }
}

function modeCaption(dep: ComponentDependencies | undefined): string | null {
  if (dep?.sourceMode === "modernize" && dep.modernizes) {
    return `Rebuilds ${dep.modernizes}`;
  }
  if (dep?.sourceMode === "importAsIs") {
    return "Vendored unmodified";
  }
  return null;
}

// Component cards: one compact single-row card per component — avatar, name and
// description. Services open their OpenAPI contract on click (JWT-guarded, so
// via the authenticated dialog, not a raw link). Onboarded components also show
// sourceMode and, for importAsIs, a Modernize CTA that seeds `/onboard <name>`.
export function ComponentsList({
  projectName,
  items,
}: {
  projectName: string;
  items: Component[];
}) {
  const [contractComponent, setContractComponent] = useState<string | null>(
    null,
  );
  const { orgHandle } = useSession();
  const dependencies = useDesignDependencies(projectName);
  const byName = new Map(
    (dependencies.data ?? []).map((c) => [c.componentName, c]),
  );

  if (items.length === 0) {
    return (
      <EmptyState
        bordered
        icon={<Boxes size={28} />}
        title="No components yet"
        description="The published plan produces them — they appear here as agents build."
      />
    );
  }

  return (
    <>
      <Stack spacing={1.5}>
        {items.map((c) => {
          const initial = ((c.displayName ?? c.name).trim()[0] ?? "C").toUpperCase();
          const openable = !isWebApp(c);
          const dep = byName.get(c.name);
          const caption = modeCaption(dep) ?? c.description ?? "—";
          const modernize = dep?.sourceMode === "importAsIs";
          const card = (
            <Card
              key={c.name}
              variant="outlined"
              {...(openable
                ? {
                    onClick: () => setContractComponent(c.name),
                    sx: {
                      cursor: "pointer",
                      transition: "border-color 120ms, box-shadow 120ms",
                      "&:hover": { borderColor: "primary.main", boxShadow: 1 },
                    },
                  }
                : {})}
            >
              <CardContent sx={{ py: 1.5, "&:last-child": { pb: 1.5 } }}>
                <Stack direction="row" spacing={2} sx={{ alignItems: "center" }}>
                  <Avatar
                    variant="rounded"
                    sx={{
                      width: 36,
                      height: 36,
                      bgcolor: "action.hover",
                      color: "text.primary",
                    }}
                  >
                    {initial}
                  </Avatar>
                  <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                    <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
                      <Typography sx={{ fontWeight: 600 }} noWrap>
                        {c.displayName ?? c.name}
                      </Typography>
                      {modeChip(dep)}
                    </Stack>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      noWrap
                      sx={{ display: "block" }}
                    >
                      {caption}
                    </Typography>
                  </Box>
                  {modernize && (
                    <Button
                      size="small"
                      variant="text"
                      onClick={(e) => {
                        e.stopPropagation();
                        setPendingSeed(
                          chatKeyFor(orgHandle ?? "default", projectName),
                          `/onboard ${c.name}`,
                        );
                      }}
                    >
                      Modernize
                    </Button>
                  )}
                </Stack>
              </CardContent>
            </Card>
          );
          return openable ? (
            <Tooltip key={c.name} title="View API contract" placement="left">
              {card}
            </Tooltip>
          ) : (
            card
          );
        })}
      </Stack>
      <ComponentOpenApiDialog
        projectName={projectName}
        componentName={contractComponent}
        onClose={() => setContractComponent(null)}
      />
    </>
  );
}

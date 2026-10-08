/**
 * symbol-detail-drawer-registration.ts — FE-CV-TASK-053-08
 *
 * Side-effect module: makes the symbol panel the Review drawer content (lazy chunk).
 * Imported by ReviewWorkspace, i.e. after the registry module has finished evaluating.
 *
 * @module components/review-map/impact/symbol-detail-drawer-registration
 */

import { lazy, Suspense } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { setReviewDrawerRenderer } from "../review-lens-registry";

const SymbolDetailPanel = lazy(() =>
  import("./SymbolDetailPanel").then((m) => ({ default: m.SymbolDetailPanel })),
);

setReviewDrawerRenderer((props) => (
  <Suspense fallback={<Skeleton className="h-24 w-full" />}>
    <SymbolDetailPanel {...props} />
  </Suspense>
));

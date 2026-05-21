import { AdminLayout } from "@/components/admin/AdminLayout";
import type { SupportBootstrap } from "@/lib/types";

type Props = { bootstrap: SupportBootstrap };

export function AdminHome({ bootstrap }: Props) {
  return <AdminLayout modules={bootstrap.modules} />;
}

import type { ReactNode } from "react";
import { Toaster } from "@/components/ui/sonner";
import { readBootstrap } from "@/lib/bootstrap";
import { AdminHome } from "@/pages/AdminHome";
import { CustomerHome } from "@/pages/CustomerHome";
import { KBBrowse } from "@/pages/kb/Browse";
import { KBDetail } from "@/pages/kb/Detail";
import { KBAdminList } from "@/pages/admin/kb/List";
import { KBAdminEdit } from "@/pages/admin/kb/Edit";
import { KBAdminCategories } from "@/pages/admin/kb/Categories";
import { KBAdminTags } from "@/pages/admin/kb/Tags";

export function App() {
  const bootstrap = readBootstrap();
  let page: ReactNode;
  switch (bootstrap.mode) {
    case "admin-home":            page = <AdminHome bootstrap={bootstrap} />; break;
    case "kb-browse":             page = <KBBrowse bootstrap={bootstrap} />; break;
    case "kb-detail":             page = <KBDetail />; break;
    case "admin-kb-list":         page = <KBAdminList />; break;
    case "admin-kb-edit":         page = <KBAdminEdit />; break;
    case "admin-kb-categories":   page = <KBAdminCategories />; break;
    case "admin-kb-tags":         page = <KBAdminTags />; break;
    default:                      page = <CustomerHome bootstrap={bootstrap} />;
  }
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}

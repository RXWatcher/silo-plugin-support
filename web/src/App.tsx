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
import { Speedtest } from "@/pages/st/Speedtest";
import { STAdminEndpoints } from "@/pages/admin/st/Endpoints";
import { STAdminGeoIP } from "@/pages/admin/st/GeoIP";
import { STAdminResults } from "@/pages/admin/st/Results";
import { STAdminDashboards } from "@/pages/admin/st/Dashboards";
import { TKList } from "@/pages/tk/List";
import { TKNew } from "@/pages/tk/New";
import { TKDetail } from "@/pages/tk/Detail";
import { TKAdminQueue } from "@/pages/admin/tk/Queue";
import { TKAdminDetail } from "@/pages/admin/tk/Detail";
import { TKAdminCategories } from "@/pages/admin/tk/Categories";

export function App() {
  const bootstrap = readBootstrap();
  let page: ReactNode;
  switch (bootstrap.mode) {
    case "admin-home":           page = <AdminHome bootstrap={bootstrap} />; break;
    case "kb-browse":            page = <KBBrowse bootstrap={bootstrap} />; break;
    case "kb-detail":            page = <KBDetail />; break;
    case "admin-kb-list":        page = <KBAdminList />; break;
    case "admin-kb-edit":        page = <KBAdminEdit />; break;
    case "admin-kb-categories":  page = <KBAdminCategories />; break;
    case "admin-kb-tags":        page = <KBAdminTags />; break;
    case "speedtest":            page = <Speedtest />; break;
    case "admin-st-endpoints":   page = <STAdminEndpoints />; break;
    case "admin-st-geoip":       page = <STAdminGeoIP />; break;
    case "admin-st-results":     page = <STAdminResults />; break;
    case "admin-st-dashboards":  page = <STAdminDashboards />; break;
    case "tickets-list":             page = <TKList />; break;
    case "tickets-new":              page = <TKNew />; break;
    case "tickets-detail":           page = <TKDetail />; break;
    case "admin-tickets-queue":      page = <TKAdminQueue />; break;
    case "admin-tickets-detail":     page = <TKAdminDetail />; break;
    case "admin-tickets-categories": page = <TKAdminCategories />; break;
    default:                         page = <CustomerHome bootstrap={bootstrap} />;
  }
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}

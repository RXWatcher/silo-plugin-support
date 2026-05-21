import { Toaster } from "@/components/ui/sonner";
import { readBootstrap } from "@/lib/bootstrap";
import { AdminHome } from "@/pages/AdminHome";
import { CustomerHome } from "@/pages/CustomerHome";

export function App() {
  const bootstrap = readBootstrap();
  const page =
    bootstrap.mode === "admin-home"
      ? <AdminHome bootstrap={bootstrap} />
      : <CustomerHome bootstrap={bootstrap} />;
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}

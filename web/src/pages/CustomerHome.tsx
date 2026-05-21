import { ModuleCard } from "@/components/shared/ModuleCard";
import { TopBar } from "@/components/shared/TopBar";
import type { SupportBootstrap } from "@/lib/types";

type Props = { bootstrap: SupportBootstrap };

export function CustomerHome({ bootstrap }: Props) {
  const m = bootstrap.modules;
  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-5xl space-y-8 px-4 py-10 md:px-8">
        <TopBar
          eyebrow="Support"
          title="Get help"
          subtitle="Browse answers, run a connection test, or open a ticket if you're stuck."
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <ModuleCard title="Knowledge Base" href="./kb"        enabled={m.kb}        description="Browse articles and FAQs." />
          <ModuleCard title="Speedtest"      href="./speedtest" enabled={m.speedtest} description="Test your connection." />
          <ModuleCard title="Tickets"        href="./tickets"   enabled={m.tickets}   description="View or open a support ticket." />
          <ModuleCard title="AI Assistant"   href="./ai"        enabled={m.ai}        description="Ask a question." />
        </div>
      </div>
    </main>
  );
}

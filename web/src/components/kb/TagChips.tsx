import { Badge } from "@/components/ui/badge";

type Props = {
  tags: string[];
  selected: string;
  onSelect: (slug: string) => void;
};

export function TagChips({ tags, selected, onSelect }: Props) {
  const chips: Array<{ slug: string; label: string }> = [
    { slug: "", label: "All" },
    ...tags.map((t) => ({ slug: t, label: t })),
  ];
  return (
    <div className="flex flex-wrap gap-2">
      {chips.map((c) => {
        const pressed = c.slug === selected;
        return (
          <button
            key={c.slug || "__all__"}
            type="button"
            aria-pressed={pressed}
            onClick={() => onSelect(c.slug)}
            className="focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-full"
          >
            <Badge variant={pressed ? "default" : "outline"}>{c.label}</Badge>
          </button>
        );
      })}
    </div>
  );
}

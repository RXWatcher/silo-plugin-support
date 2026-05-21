import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";

import { Input } from "@/components/ui/input";

type Props = {
  onQuery: (q: string) => void;
  debounceMs?: number;
  initialValue?: string;
};

export function SearchBar({ onQuery, debounceMs = 250, initialValue = "" }: Props) {
  const [value, setValue] = useState(initialValue);
  const timer = useRef<number | null>(null);

  useEffect(() => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => onQuery(value), debounceMs);
    return () => {
      if (timer.current) window.clearTimeout(timer.current);
    };
  }, [value, onQuery, debounceMs]);

  return (
    <div className="relative">
      <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        type="search"
        role="searchbox"
        className="pl-8"
        placeholder="Search articles..."
        value={value}
        onChange={(e) => setValue(e.target.value)}
      />
    </div>
  );
}

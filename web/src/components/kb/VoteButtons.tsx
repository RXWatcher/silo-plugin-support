import { ThumbsDown, ThumbsUp } from "lucide-react";

import { Button } from "@/components/ui/button";

type Props = {
  currentVote: "up" | "down" | null;
  onVote: (v: "up" | "down") => void;
};

export function VoteButtons({ currentVote, onVote }: Props) {
  return (
    <div className="flex items-center gap-3">
      <p className="text-sm text-muted-foreground">Was this helpful?</p>
      <Button
        type="button"
        variant={currentVote === "up" ? "default" : "outline"}
        size="sm"
        aria-pressed={currentVote === "up"}
        onClick={() => onVote("up")}
      >
        <ThumbsUp className="mr-2 h-4 w-4" /> Helpful
      </Button>
      <Button
        type="button"
        variant={currentVote === "down" ? "default" : "outline"}
        size="sm"
        aria-pressed={currentVote === "down"}
        onClick={() => onVote("down")}
      >
        <ThumbsDown className="mr-2 h-4 w-4" /> Not helpful
      </Button>
    </div>
  );
}

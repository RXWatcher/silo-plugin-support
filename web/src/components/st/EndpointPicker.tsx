import type { STEndpoint } from "@/lib/types";

type Props = {
  endpoints: STEndpoint[];
  value: "auto" | number;
  onChange: (v: "auto" | number) => void;
  disabled?: boolean;
};

export function EndpointPicker({ endpoints, value, onChange, disabled }: Props) {
  return (
    <select
      role="combobox"
      disabled={disabled}
      value={value === "auto" ? "auto" : String(value)}
      onChange={(e) => {
        const v = e.target.value;
        onChange(v === "auto" ? "auto" : Number(v));
      }}
      className="rounded border border-border bg-background px-2 py-1 text-sm"
    >
      <option value="auto">Auto</option>
      {endpoints.map((e) => (
        <option key={e.id} value={String(e.id)}>{e.label}{e.country ? ` (${e.country})` : ""}</option>
      ))}
    </select>
  );
}

import React from "react";
import { useColorMode, useThemeConfig } from "@docusaurus/theme-common";
import styles from "./styles.module.css";

type Choice = "light" | "dark" | null;

const options: Array<{ value: Choice; label: string; symbol: string }> = [
  { value: "light", label: "Light theme", symbol: "☀" },
  { value: null, label: "Use system theme", symbol: "◐" },
  { value: "dark", label: "Dark theme", symbol: "☾" },
];

export default function NavbarColorModeToggle({
  className,
}: {
  className?: string;
}) {
  const { disableSwitch } = useThemeConfig().colorMode;
  const { colorModeChoice, setColorMode } = useColorMode();

  if (disableSwitch) return null;

  return (
    <div
      className={`${styles.switcher} ${className || ""}`}
      aria-label="Color theme"
    >
      {options.map((option) => {
        const active = colorModeChoice === option.value;
        return (
          <button
            key={option.label}
            type="button"
            className={active ? styles.active : undefined}
            aria-label={option.label}
            aria-pressed={active}
            title={option.label}
            onClick={() => setColorMode(option.value)}
          >
            <span aria-hidden="true">{option.symbol}</span>
          </button>
        );
      })}
    </div>
  );
}

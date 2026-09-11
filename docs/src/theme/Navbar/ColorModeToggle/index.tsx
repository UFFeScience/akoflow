import React, {type ReactNode} from "react";
import { useColorMode, useThemeConfig } from "@docusaurus/theme-common";
import styles from "./styles.module.css";

type Choice = "light" | "dark" | null;

const iconProps = {
  width: 14,
  height: 14,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.8,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

const options: Array<{ value: Choice; label: string; icon: ReactNode }> = [
  {
    value: null,
    label: "Use system theme",
    icon: <svg {...iconProps}><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8M12 17v4"/></svg>,
  },
  {
    value: "light",
    label: "Light theme",
    icon: <svg {...iconProps}><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.42-1.42M17.66 6.34l1.41-1.41"/></svg>,
  },
  {
    value: "dark",
    label: "Dark theme",
    icon: <svg {...iconProps}><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z"/></svg>,
  },
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
            <span aria-hidden="true">{option.icon}</span>
          </button>
        );
      })}
    </div>
  );
}

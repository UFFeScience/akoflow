import React from "react";
import { useColorMode } from "@docusaurus/theme-common";
import styles from "./blog.module.css";

export default function BlogThemeSwitch() {
  const { colorMode, setColorMode } = useColorMode();
  return (
    <div
      className={styles.themeSwitch}
      role="group"
      aria-label="Blog appearance"
    >
      {(["light", "dark"] as const).map((mode) => (
        <button
          key={mode}
          type="button"
          aria-pressed={colorMode === mode}
          onClick={() => setColorMode(mode)}
        >
          {mode === "light" ? "☼ Light" : "☾ Dark"}
        </button>
      ))}
    </div>
  );
}

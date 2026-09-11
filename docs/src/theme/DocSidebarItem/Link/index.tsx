import React, {type ReactNode} from "react";
import clsx from "clsx";
import {ThemeClassNames} from "@docusaurus/theme-common";
import {isActiveSidebarItem} from "@docusaurus/plugin-content-docs/client";
import Link from "@docusaurus/Link";
import isInternalUrl from "@docusaurus/isInternalUrl";
import IconExternalLink from "@theme/Icon/ExternalLink";
import type {Props} from "@theme/DocSidebarItem/Link";

import styles from "./styles.module.css";

const endpointLabelPattern = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(.+)$/;

function EndpointLabel({label}: {label: ReactNode}) {
  if (typeof label !== "string") return label;

  const match = endpointLabelPattern.exec(label);
  if (!match) return label;

  const [, method, path] = match;
  return (
    <span className={styles.endpoint}>
      <span
        className={clsx(styles.method, styles[`method${method}`])}
        aria-hidden="true"
      >
        {method}
      </span>
      <span className={styles.path}>{path}</span>
    </span>
  );
}

export default function DocSidebarItemLink({
  item,
  onItemClick,
  activePath,
  level,
  index,
  ...props
}: Props): ReactNode {
  const {href, label, className, autoAddBaseUrl} = item;
  const isActive = isActiveSidebarItem(item, activePath);
  const isInternalLink = isInternalUrl(href);

  return (
    <li
      className={clsx(
        ThemeClassNames.docs.docSidebarItemLink,
        ThemeClassNames.docs.docSidebarItemLinkLevel(level),
        "menu__list-item",
        className,
      )}
      key={String(label)}
    >
      <Link
        className={clsx(
          "menu__link",
          !isInternalLink && styles.menuExternalLink,
          {"menu__link--active": isActive},
        )}
        autoAddBaseUrl={autoAddBaseUrl}
        aria-current={isActive ? "page" : undefined}
        aria-label={typeof label === "string" ? label : undefined}
        to={href}
        {...(isInternalLink && {
          onClick: onItemClick ? () => onItemClick(item) : undefined,
        })}
        {...props}
      >
        <EndpointLabel label={label} />
        {!isInternalLink && <IconExternalLink />}
      </Link>
    </li>
  );
}

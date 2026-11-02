import React, { type ReactNode } from "react";
import { useBlogPost } from "@docusaurus/plugin-content-blog/client";
import OriginalFooter from "@theme-original/BlogPostItem/Footer";
import EditMetaRow from "@theme/EditMetaRow";

export default function BlogPostItemFooter(): ReactNode {
  const { metadata, isBlogPostPage } = useBlogPost();

  if (!isBlogPostPage) {
    return <OriginalFooter />;
  }

  const { editUrl, lastUpdatedAt, lastUpdatedBy } = metadata;

  if (!editUrl && !lastUpdatedAt && !lastUpdatedBy) {
    return null;
  }

  return (
    <footer className="docusaurus-mt-lg">
      <EditMetaRow
        className="margin-top--sm"
        editUrl={editUrl}
        lastUpdatedAt={lastUpdatedAt}
        lastUpdatedBy={lastUpdatedBy}
      />
    </footer>
  );
}

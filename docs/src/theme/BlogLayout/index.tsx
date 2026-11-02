import React from "react";
import Layout from "@theme/Layout";
import type { Props } from "@theme/BlogLayout";
import BlogBackdrop from "@site/src/components/blog/BlogBackdrop";
import BlogLinkHighlights from "@site/src/components/blog/BlogLinkHighlights";
import styles from "@site/src/components/blog/blog.module.css";

export default function BlogLayout({
  children,
  toc,
  sidebar,
  ...props
}: Props) {
  return (
    <Layout {...props}>
      <div className={styles.shell} data-blog-link-highlights>
        <BlogBackdrop />
        <BlogLinkHighlights />
        <div className={styles.articleLayout}>
          <main>{children}</main>
          {toc && <aside className={styles.toc}>{toc}</aside>}
        </div>
      </div>
    </Layout>
  );
}

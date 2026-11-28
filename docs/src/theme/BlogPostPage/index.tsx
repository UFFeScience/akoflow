import React from "react";
import Head from "@docusaurus/Head";
import Link from "@docusaurus/Link";
import { useLocation } from "@docusaurus/router";
import useIsBrowser from "@docusaurus/useIsBrowser";
import Layout from "@theme/Layout";
import BlogPostPage from "@theme-original/BlogPostPage";
import type { Props } from "@theme/BlogPostPage";
import { isBlogPreview, isScheduledBlogPost } from "@site/src/utils/blogSchedule";

export default function ScheduledBlogPostPage(props: Props) {
  const { search } = useLocation();
  const isBrowser = useIsBrowser();
  const { frontMatter, unlisted } = props.content.metadata;
  const scheduled = frontMatter.publish_at !== undefined &&
    (isBrowser ? isScheduledBlogPost(frontMatter) : unlisted);
  if (!scheduled || (isBrowser && isBlogPreview(search))) {
    return <BlogPostPage {...props} />;
  }

  return (
    <Layout title="Scheduled article">
      <Head><meta name="robots" content="noindex, nofollow" /></Head>
      <main className="container margin-vert--xl">
        <h1>This article is scheduled</h1>
        <Link to="/blog">Back to the blog</Link>
      </main>
    </Layout>
  );
}

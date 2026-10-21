import React from "react";
import FooterLayout from "@theme-original/Footer/Layout";
import type { Props } from "@theme/Footer/Layout";
import FooterBrands from "@site/src/components/footer/FooterBrands";

export default function BrandedFooterLayout({ links, ...props }: Props) {
  return (
    <FooterLayout
      {...props}
      links={
        <>
          <FooterBrands />
          {links}
        </>
      }
    />
  );
}

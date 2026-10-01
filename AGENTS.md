# AkôFlow repository guidance

## Documentation diagrams

- Use Mermaid for architecture, lifecycle, hierarchy, relationship, state, and flow diagrams in the documentation.
- Keep diagrams monochromatic. Use `#151515` for text, borders, arrows, and emphasized fills; `#ffffff` for the page and primary nodes; and `#f2f2f2` for neutral secondary nodes.
- When an emphasized node uses a `#151515` fill, use `#ffffff` text.
- Do not use Mermaid's default colored node palette. Define and apply `classDef` styles explicitly, and set `linkStyle default stroke:#151515,stroke-width:2px,color:#151515`.
- Keep the global Mermaid theme in `docs/docusaurus.config.ts` and the responsive centering rules in `docs/src/css/custom.css` intact so unclassified diagrams follow the same palette and alignment automatically.
- Prefer one Mermaid source in the Markdown or MDX page. Do not maintain a duplicate SVG and Mermaid version of the same diagram.
- Keep labels short, preserve meaningful arrow direction, and verify the rendered diagram at desktop and narrow widths.
- Use a static SVG only when Mermaid cannot express the visual faithfully, such as a branded illustration or a precise quantitative graphic. Document the exception next to the source.

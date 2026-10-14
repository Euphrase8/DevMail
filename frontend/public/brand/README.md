# DevMail brand assets

The transparent SVG artwork is traced from the supplied `devmail.jpg` logo. It
preserves the envelope/code/server symbol and the original wordmark outlines;
there is no embedded bitmap, background rectangle, or font dependency.

- `devmail-logo-light.svg` uses charcoal lettering on light surfaces.
- `devmail-logo-dark.svg` uses ivory lettering on dark surfaces.
- `devmail-mark.svg` contains only the envelope symbol.

Use the shared `Logo` and `BrandMark` React components for app branding. The
favicon SVG, 16/32px PNGs, multi-size ICO, Apple touch icon and manifest icons
all contain only the mail symbol, with transparent backgrounds. The ICO holds
16, 32 and 48px sizes. PNG/ICO exports are rendered from `favicon.svg`.

Imagegen background-extraction drafts were evaluated with the prompt to remove
only the dark background, preserve the exact logo and clean the edges. They
introduced edge artifacts, so the shipped assets use a direct vector trace of
the supplied source instead (Potrace, threshold 65, blackOnWhite false,
turdSize 6, optTolerance 0.35), with the original coral palette and theme-aware
neutral lettering. The draft images are not used by the app.

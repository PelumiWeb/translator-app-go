import { ImageResponse } from "next/og";
import { PROMISE, PROMISE_LINE, ROSA, WORDMARK } from "@/lib/site";

// The picture a link to the site shows in WhatsApp, iMessage, X or LinkedIn
// (app/layout.tsx names it): the promise, and Rosa's note, Spanish on her
// side, English on his. Drawn once, at build time, from the same words as the
// page, so the name still lives only in lib/site.ts. A route named og.png
// rather than Next's opengraph-image file, which a static export writes
// without an extension, so a host would not know to serve it as a PNG.

export const dynamic = "force-static";
const size = { width: 1200, height: 630 };

const C = {
  bg: "#0E2426",
  raised: "#173539",
  deep: "#071416",
  fg: "#F3EEE4",
  fg2: "rgba(243, 238, 228, 0.64)",
  them: "#7FD1C1",
  lens: "#B9E0D3",
};

// The picture needs the font's file itself, which next/font does not hand
// over, so it is fetched from Google Fonts while building, as next/font does
// for the page. Offline, the picture falls back to the default font rather
// than stopping the build.
async function plex(weight: 400 | 500, text: string): Promise<ArrayBuffer | null> {
  try {
    const css = await (
      await fetch(`https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@${weight}&text=${encodeURIComponent(text)}`)
    ).text();
    const url = css.match(/src: url\((.+?)\) format\('(opentype|truetype)'\)/)?.[1];
    if (!url) return null;
    const font = await fetch(url);
    return font.ok ? await font.arrayBuffer() : null;
  } catch {
    return null;
  }
}

const LEVELS = [4, 6, 3, 8, 10, 7, 5, 9, 6, 4, 8, 10, 6, 3, 7, 9, 5, 4, 8, 6, 3, 5];

export async function GET() {
  const words = [WORDMARK, PROMISE, PROMISE_LINE, ROSA.spanishShort, ROSA.englishShort, "In her voice", ROSA.duration].join(" ");
  const [regular, medium] = await Promise.all([plex(400, words), plex(500, words)]);
  const fonts = [
    ...(regular ? [{ name: "Plex", data: regular, weight: 400 as const }] : []),
    ...(medium ? [{ name: "Plex", data: medium, weight: 500 as const }] : []),
  ];
  if (fonts.length < 2) console.warn("opengraph-image: IBM Plex Sans could not be fetched; the default font is used");

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          padding: 72,
          background: C.bg,
          color: C.fg,
          fontFamily: "Plex",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", justifyContent: "space-between", width: 600 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <svg viewBox="6 16 108 50" width={80} height={37}>
              <line x1="8" y1="62" x2="112" y2="62" stroke={C.fg} strokeOpacity="0.5" strokeWidth="2.5" strokeLinecap="round" />
              <circle cx="46" cy="40" r="22" fill={C.them} />
              <circle cx="74" cy="40" r="22" fill={C.fg} />
              <path d="M60 23.03A22 22 0 0 1 60 56.97A22 22 0 0 1 60 23.03Z" fill={C.lens} />
            </svg>
            <span style={{ fontSize: 40, fontWeight: 500, letterSpacing: "-0.04em" }}>{WORDMARK}</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
            <span style={{ fontSize: 60, fontWeight: 500, lineHeight: 1.08, letterSpacing: "-0.02em" }}>{PROMISE}</span>
            <span style={{ fontSize: 26, lineHeight: 1.4, color: C.fg2 }}>{PROMISE_LINE}</span>
          </div>
        </div>

        <div style={{ display: "flex", flexDirection: "column", justifyContent: "center", gap: 20, marginLeft: 56, width: 400 }}>
          {/* Hers: warm white, in Spanish. */}
          <div
            style={{
              display: "flex",
              alignSelf: "flex-end",
              width: 340,
              padding: "18px 22px",
              borderRadius: 14,
              background: C.fg,
              color: C.bg,
              fontSize: 24,
              lineHeight: 1.4,
            }}
          >
            {ROSA.spanishShort}
          </div>
          {/* His: raised petrol, mint, in English, in her voice. */}
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              width: 380,
              padding: "20px 22px",
              borderRadius: 14,
              background: C.raised,
              gap: 14,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  width: 52,
                  height: 52,
                  borderRadius: 26,
                  background: C.them,
                }}
              >
                <svg width="20" height="20" viewBox="0 0 24 24">
                  <path d="M7 4.6v14.8a1 1 0 0 0 1.52.86l12.2-7.4a1 1 0 0 0 0-1.72L8.52 3.74A1 1 0 0 0 7 4.6Z" fill={C.bg} />
                </svg>
              </div>
              <div style={{ display: "flex", flex: 1, alignItems: "center", justifyContent: "space-between" }}>
                {LEVELS.map((d, i) => (
                  <div
                    key={i}
                    style={{ width: d, height: d, borderRadius: d, background: C.them, opacity: i < 9 ? 1 : 0.3 }}
                  />
                ))}
              </div>
              <span style={{ fontSize: 16, color: C.fg2 }}>{ROSA.duration}</span>
            </div>
            <span style={{ fontSize: 24, lineHeight: 1.4 }}>{ROSA.englishShort}</span>
            <span style={{ fontSize: 16, color: C.fg2 }}>In her voice</span>
          </div>
        </div>
      </div>
    ),
    { ...size, fonts },
  );
}

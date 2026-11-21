import type { Metadata, Viewport } from "next";
import { IBM_Plex_Sans } from "next/font/google";
import { MotionProvider } from "@/components/MotionProvider";
import { APP_NAME, PROMISE, PROMISE_LINE, SITE_URL } from "@/lib/site";
import "./globals.css";

// Downloaded at build time and served from the site itself, so visitors'
// browsers never contact Google. latin-ext covers Yorùbá's and Hausa's letters.
const plex = IBM_Plex_Sans({
  weight: ["400", "500"],
  subsets: ["latin", "latin-ext"],
  display: "swap",
  variable: "--font-plex",
});

// Drawn by app/og.png/route.tsx.
const OG_IMAGE = {
  url: "/og.png",
  width: 1200,
  height: 630,
  alt: `${APP_NAME}: ${PROMISE} Rosa's voice note, said in Spanish, arrives in English, in her voice.`,
};

export const metadata: Metadata = {
  // Link previews fetch the picture by its full address, which starts here.
  metadataBase: new URL(SITE_URL),
  title: `${APP_NAME}: ${PROMISE}`,
  description: PROMISE_LINE,
  openGraph: {
    title: APP_NAME,
    description: `${PROMISE} ${PROMISE_LINE}`,
    type: "website",
    images: [OG_IMAGE],
  },
  twitter: {
    card: "summary_large_image",
    images: [OG_IMAGE],
  },
};

export const viewport: Viewport = {
  themeColor: "#0E2426",
  colorScheme: "dark",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={plex.variable}>
      <body>
        <MotionProvider>{children}</MotionProvider>
      </body>
    </html>
  );
}

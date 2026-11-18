import type { Metadata, Viewport } from "next";
import { IBM_Plex_Sans } from "next/font/google";
import { APP_NAME, PROMISE, PROMISE_LINE } from "@/lib/site";
import "./globals.css";

// Downloaded at build time and served from the site itself, so visitors'
// browsers never contact Google. latin-ext covers Yorùbá's and Hausa's letters.
const plex = IBM_Plex_Sans({
  weight: ["400", "500"],
  subsets: ["latin", "latin-ext"],
  display: "swap",
  variable: "--font-plex",
});

export const metadata: Metadata = {
  title: `${APP_NAME}: ${PROMISE}`,
  description: PROMISE_LINE,
  openGraph: {
    title: APP_NAME,
    description: `${PROMISE} ${PROMISE_LINE}`,
    type: "website",
  },
};

export const viewport: Viewport = {
  themeColor: "#0E2426",
  colorScheme: "dark",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={plex.variable}>
      <body>{children}</body>
    </html>
  );
}

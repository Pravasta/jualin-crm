import type { Metadata } from "next";
import { JetBrains_Mono, Plus_Jakarta_Sans } from "next/font/google";
import "./globals.css";

// One UI family (design brief §5.3: performance) plus a monospace for lead
// numbers, phone numbers, API keys and embed snippets — Phase 8.6 token
// sheet. Weights are the ones the design actually uses; each extra weight
// is another font file on first load.
const plusJakartaSans = Plus_Jakarta_Sans({
  variable: "--font-plus-jakarta-sans",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
});

const jetBrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
  weight: ["400", "500"],
});

export const metadata: Metadata = {
  title: "Jualin CRM",
  description: "Dashboard CRM untuk Owner, Admin, dan Manager.",
};

// lang="id" tanpa library i18n — keputusan C1 (docs/phases/03-owner-dashboard/prd.md):
// UI seluruhnya Bahasa Indonesia, konsisten dengan error.message backend
// yang sudah Bahasa Indonesia sejak issue #9.
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="id"
      className={`${plusJakartaSans.variable} ${jetBrainsMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}

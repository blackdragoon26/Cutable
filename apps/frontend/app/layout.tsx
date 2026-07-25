import type { Metadata } from "next";
import "./globals.css";
import { QueryProvider } from "./providers/QueryProvider";

export const metadata: Metadata = {
  title: "Cutable - Build beautiful websites in a single prompt",
  description: "Learn smarter, faster, and more interactively with our custom AI model.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased">
        <QueryProvider>{children}</QueryProvider>
      </body>
    </html>
  );
}

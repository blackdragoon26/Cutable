"use client";

import Image from "next/image";
import Link from "next/link";
import { useRef } from "react";

type BrandProps = {
  compact?: boolean;
  className?: string;
};

export default function Brand({ compact = false, className = "" }: BrandProps) {
  const videoRef = useRef<HTMLVideoElement>(null);

  const playLogo = () => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    void videoRef.current?.play();
  };

  const resetLogo = () => {
    const video = videoRef.current;
    if (!video) return;
    video.pause();
    video.currentTime = 0;
  };

  return (
    <Link
      href="/"
      aria-label="Cutable home"
      onMouseEnter={playLogo}
      onMouseLeave={resetLogo}
      onFocus={playLogo}
      onBlur={resetLogo}
      className={`group inline-flex items-center font-semibold tracking-tight text-stone-950 ${className}`}
    >
      <span
        className={`relative mr-2.5 shrink-0 ${
          compact ? "h-8 w-8" : "h-9 w-9"
        }`}
        aria-hidden="true"
      >
        <Image
          src="/brand/cutable-mark-v3.png"
          alt=""
          fill
          sizes={compact ? "32px" : "36px"}
          className="object-contain"
          priority
        />
        <video
          ref={videoRef}
          muted
          loop
          playsInline
          preload="metadata"
          poster="/brand/cutable-mark-v3.png"
          className="absolute inset-0 h-full w-full object-contain opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100"
        >
          <source src="/brand/cutable-hover.webm" type="video/webm" />
        </video>
      </span>
      <span className={compact ? "text-sm" : "text-[17px]"}>Cutable</span>
    </Link>
  );
}

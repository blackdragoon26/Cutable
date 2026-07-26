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
        className={`relative mr-2 shrink-0 overflow-hidden rounded-md bg-[#d9d9d7] ${
          compact ? "h-7 w-7" : "h-8 w-8"
        }`}
        aria-hidden="true"
      >
        <Image
          src="/brand/cutable-mark.png"
          alt=""
          fill
          sizes={compact ? "28px" : "32px"}
          className="object-cover"
          priority
        />
        <video
          ref={videoRef}
          muted
          loop
          playsInline
          preload="metadata"
          poster="/brand/cutable-mark.png"
          className="absolute inset-0 h-full w-full object-cover opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100"
        >
          <source src="/brand/cutable-hover.mp4" type="video/mp4" />
        </video>
      </span>
      <span className={compact ? "text-sm" : "text-base"}>Cutable</span>
    </Link>
  );
}

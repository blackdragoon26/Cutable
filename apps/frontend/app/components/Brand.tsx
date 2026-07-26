import Image from "next/image";
import Link from "next/link";

type BrandProps = {
  compact?: boolean;
  className?: string;
};

export default function Brand({ compact = false, className = "" }: BrandProps) {
  return (
    <Link
      href="/"
      aria-label="Cutable home"
      className={`inline-flex items-center font-semibold tracking-tight text-stone-950 ${className}`}
    >
      <Image
        src="/brand/cutable-mark.png"
        alt=""
        width={compact ? 25 : 30}
        height={compact ? 24 : 29}
        className={compact ? "mr-1.5 h-6 w-auto object-contain" : "mr-2 h-7 w-auto object-contain"}
        priority
      />
      <span className={compact ? "text-sm" : "text-base"}>Cutable</span>
    </Link>
  );
}

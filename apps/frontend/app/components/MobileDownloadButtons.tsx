const ANDROID_APK_URL =
  "https://github.com/blackdragoon26/Cutable/releases/download/mobile-latest/cutable.apk";

export default function MobileDownloadButtons() {
  return (
    <div className="flex flex-col items-center gap-3 text-center sm:flex-row">
      <span
        className="inline-flex cursor-default items-center gap-2 rounded-lg border border-stone-300 bg-white px-4 py-2.5 text-sm font-medium text-stone-700 shadow-sm"
        title="iOS TestFlight beta is coming soon"
      >
        <span className="h-2 w-2 rounded-full bg-stone-300" />
        Join TestFlight — Coming soon
      </span>
      <a
        href={ANDROID_APK_URL}
        className="inline-flex items-center gap-2 rounded-lg border border-stone-300 bg-white px-4 py-2.5 text-sm font-medium text-stone-700 shadow-sm transition hover:border-stone-400 hover:bg-stone-50"
      >
        <span className="h-2 w-2 rounded-full bg-emerald-500" />
        Download for Android (APK)
      </a>
    </div>
  );
}

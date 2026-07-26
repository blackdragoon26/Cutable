import HeroSection from "./components/HeroSection";
import Header from "./components/Header";
import SakuraField from "./components/SakuraField";

export default function Home() {
  return (
    <>
      <Header />
      <main className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#f6f6f3] pt-20">
        <SakuraField />
        <div className="relative z-10 w-full">
          <HeroSection />
        </div>
      </main>
    </>
  );
}

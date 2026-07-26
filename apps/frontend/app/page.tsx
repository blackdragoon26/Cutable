import HeroSection from "./components/HeroSection";
import Header from "./components/Header";

export default function Home() {
  return (
    <>
      <Header />
      <main className="flex min-h-screen items-center justify-center bg-[#f6f6f3] pt-20">
        <HeroSection />
      </main>
    </>
  );
}

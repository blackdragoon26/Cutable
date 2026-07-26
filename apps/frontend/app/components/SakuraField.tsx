import type { CSSProperties } from "react";

const petals = [
  [4, 0, 12, 0.72],
  [10, 5, 15, 0.55],
  [17, 2, 11, 0.8],
  [24, 8, 17, 0.5],
  [31, 1, 13, 0.65],
  [39, 6, 18, 0.45],
  [48, 3, 14, 0.76],
  [56, 9, 16, 0.52],
  [64, 0, 12, 0.7],
  [71, 7, 19, 0.46],
  [79, 2, 14, 0.74],
  [86, 10, 16, 0.5],
  [93, 4, 13, 0.68],
] as const;

export default function SakuraField() {
  return (
    <div className="sakura-field" aria-hidden="true">
      {petals.map(([left, delay, duration, opacity], index) => (
        <span
          key={`${left}-${delay}`}
          className="sakura-petal"
          style={
            {
              "--petal-left": `${left}%`,
              "--petal-delay": `-${delay}s`,
              "--petal-duration": `${duration}s`,
              "--petal-opacity": opacity,
              "--petal-drift": `${index % 2 ? 96 : -72}px`,
            } as CSSProperties
          }
        />
      ))}
    </div>
  );
}

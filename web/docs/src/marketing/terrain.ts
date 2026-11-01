import { contourDensity } from "d3-contour";
import { geoPath } from "d3-geo";

/** One isoline of a terrain: SVG path data, and whether it is an index (every fifth) line. */
export interface TerrainLine {
  d: string;
  index: boolean;
}

/**
 * A deterministic terrain for a survey drawing: each point raises a hill, a scatter of lighter
 * points around it keeps the ground from reading as rings, and d3-contour draws the isolines.
 * Used wherever the marketing site draws data as a surveyed map, so the ground looks the same.
 */
export function terrain(
  points: { x: number; y: number }[],
  width: number,
  height: number,
  { seed = 7, spread = 240, bandwidth = 44, thresholds = 15 } = {},
): TerrainLine[] {
  let s = seed;
  const rand = () => ((s = (s * 16807) % 2147483647) - 1) / 2147483646;
  const ground: [number, number, number][] = [];
  for (const p of points) {
    ground.push([p.x, p.y, 6]);
    for (let i = 0; i < 14; i++) ground.push([p.x + (rand() - 0.5) * spread, p.y + (rand() - 0.5) * spread * 0.85, 1]);
  }
  for (let i = 0; i < 60; i++) ground.push([rand() * width, rand() * height, 0.6]);
  const path = geoPath();
  return contourDensity<[number, number, number]>()
    .x((d) => d[0])
    .y((d) => d[1])
    .weight((d) => d[2])
    .size([width, height])
    .cellSize(4)
    .bandwidth(bandwidth)
    .thresholds(thresholds)(ground)
    .map((c, i) => ({ d: path(c) ?? "", index: i % 5 === 4 }));
}

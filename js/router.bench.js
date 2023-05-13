import { Router } from './router.js';
import { readFile } from 'fs/promises';

async function bench(n) {
  const json = await readFile("bench_data/routes" + n, "utf-8");
  const router = new Router(JSON.parse(json));

  const nRoutesToProbe = 10;
  const routeStrings = new Array(nRoutesToProbe);
  for (let i = 0; i < nRoutesToProbe; ++i) {
    const rn = i * Math.floor(n/nRoutesToProbe);
    const r = "/" + rn + "foo";
    routeStrings[i] = r;
  }

  const nIterations = 10000;
  const start = performance.now();
  for (let i = 0; i < nIterations; ++i) {
    for (let j = 0; j < nRoutesToProbe; ++j) {
      router.route(routeStrings[j]);
    }
  }
  const end = performance.now();
  const tot = end - start;
  const per = tot/(nIterations * nRoutesToProbe);

  console.log(`${n}: ${per.toFixed(4)} milliseconds per routing operation`);
}

await bench(10);
await bench(100);
await bench(1000);
await bench(10000);

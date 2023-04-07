import { Router } from './router.js';
import { readFile } from 'fs/promises';

async function bench(n) {
  const json = await readFile("bench_data/routes" + n, "utf-8");
  const router = new Router(JSON.parse(json));

  const routeStrings = new Array(10);
  for (let i = 0; i < 10; ++i) {
    const rn = i * Math.floor(n/10);
    const r = "/" + rn + "foo";
    routeStrings[i] = r;
  }

  const start = performance.now();
  for (let i = 0; i < 10000; ++i) {
    for (let j = 0; j < routeStrings.length; ++j) {
      router.route(routeStrings[j]);
    }
  }
  const end = performance.now();
  const tot = end - start;
  const per = tot/(10000 * 10);

  console.log(`${n}: ${per} milliseconds per routing operation`);
}

await bench(10);
await bench(100);
await bench(1000);
await bench(10000);

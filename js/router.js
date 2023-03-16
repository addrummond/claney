export class Router {
  constructor(json) {
    this.json = json;

    this.cpr = new RegExp(json.constantPortionRegexp);
    this.groupRegexps = { };
    for (const cp of Object.keys(json.families)) {
      this.groupRegexps[cp] = new RegExp(json.families[cp].matchRegexp);
    }
  }

  route(url) {
    const m = url.match(this.cpr);
    if (m === null)
      return null;

    // join can hopefully do a better job iterating over the sparse array than
    // we could by manually incrementing an index.
    const cp = m.join('').substring(m[0].length);

    const family = this.json.families[cp];
    if (family === undefined)
      return null;

    const submatches = url.match(this.groupRegexps[cp]);
    if (submatches === null)
      return null;
  
    const groupIndex = this.#findGroupIndex(submatches, family.nonparamGroupNumbers, family.nLevels);

    let params = { };
    const member = family.members[groupIndex];
    for (const [name, val] of Object.entries(member.paramGroupNumbers)) {
      params[name] = submatches[val];
    }

    const query = submatches[submatches.length-2] || "";
    const anchor = submatches[submatches.length-1] || "";

    return {
      name: member.name,
      params,
      query,
      anchor,
      tags: member.tags,
      methods: member.methods
    };
  }

  #findGroupIndex(match, nonParamGroupNumbers, nLevels) {
    // binary search
    let mi = 0; // start of match group range
    let nLeaves = 2 ** (nLevels-1);
    let gi = 0;

    for (let l = 0; l < nLevels; ++l, nLeaves >>= 1) {
      if (match[nonParamGroupNumbers[mi]] === undefined) {
        // take the right branch
        gi += nLeaves;
        mi += nLeaves*2;
      } else {
        // take the left branch
        ++mi;
      }
    }

    return gi;
  }
}

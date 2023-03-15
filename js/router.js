export class Router {
  constructor(json) {
    this.json = json;

    this.cpr = new RegExp(json.constantPortionRegexp);
    this.groupRegexes = { };
    for (const cp of Object.keys(json.families)) {
      this.groupRegexes[cp] = new RegExp(json.families[cp].matchRegexp);
    }
  }

  route(url) {
    // We can't easily use '.replace' in Javascript because backreferences to
    // match groups (e.g. '$12') max out at two digits. ES2018 introduces named
    // capture groups. We could mung the regex to name each capture group (e.g.
    // '(?<g1>...', '(?<g2>', ...), and then refer back to an unlimited number of
    // capture groups by name. However, this would possibly limit browser
    // compat. Instead we just manually join the matches into a string.
    const m = url.match(this.cpr);
    if (m === null)
      return null;

    let cp = '';
    for (let i = 1; i <= this.json.constantPortionNGroups; ++i) {
      if (m[i] !== undefined)
        cp += m[i];
    }

    const family = this.json.families[cp];
    if (family === undefined)
      return null;

    const submatches = url.match(this.groupRegexes[cp]);
    if (submatches === null)
      throw new Error("Internal error in 'route': constant portion regexp matched, but route-specific regexp didn't. This shouldn't happen.");
  
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

export class Router {
  constructor(json) {
    this.json = json;

    // Because JS limits us to 100 numbered backreferences, we have to name the
    // capture groups in the constant portion regexp.
    let gi = 1;
    const cpr =
      json.constantPortionRegexp
      .replace(
        /(^|[^\\])\(([^?])/g,
        (_, bef, af) => bef + '(?<g' + (gi++) + '>' + af
      )

    this.cpr = new RegExp(cpr, "d");
    this.groupRegexes = { };
    for (const cp of Object.keys(json.families)) {
      this.groupRegexes[cp] = new RegExp(json.families[cp].matchRegexp, "d");
    }
    this.repl = " "; // pad output with arbitrary additional initial char so that output will never be equal to input
    for (let i = 1; i <= json.constantPortionNGroups; ++i) {
      this.repl += '$<g' + i + '>';
    }
  }

  route(url) {
    let cp = url.replace(this.cpr, this.repl);
    if (cp === url)
      return null;
    cp = cp.substring(1) // remove initial padding char

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
      const [from, to] = submatches.indices[val];
      params[name] = url.substring(from, to);
    }

    const [qfrom, qto] = submatches.indices[submatches.indices.length-2] || [0, 0];
    const [afrom, ato] = submatches.indices[submatches.indices.length-1] || [0, 0];

    return {
      name: member.name,
      params,
      query: url.substring(qfrom, qto),
      anchor: url.substring(afrom, ato),
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
      if (match.indices[nonParamGroupNumbers[mi]] === undefined) {
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

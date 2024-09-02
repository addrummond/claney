// Equivalent to router.js, but written using minimal ES6+ features in order
// to be compatible with the CloudFront functions JS 2.0 runtime.

function Router(json, caseSensitive) {
  caseSensitive = !!caseSensitive;

  this.json = json;

  this.cpr = new RegExp(json.constantPortionRegexp);
  const groupRegexps = { };
  this.groupRegexps = groupRegexps;
  Object.keys(json.families).forEach(function (cp) {
    groupRegexps[cp] = new RegExp(json.families[cp].matchRegexp);
  });

  this.route = function (url) {
    url = normalizeUrl(url)

    const m = url.match(this.cpr);
    if (m === null)
      return null;

    // join can do a better job iterating over the sparse array than we could by
    // manually incrementing an index (confirmed by benchmarking).
    const cp = m.join('').substring(m[0].length);

    const family = this.json.families[cp];
    if (family === undefined)
      return null;

    const submatches = url.match(this.groupRegexps[cp]);
    if (submatches === null)
      return null;
  
    const groupIndex = findGroupIndex(submatches, family.nonparamGroupNumbers, family.nLevels);

    let params = { };
    const member = family.members[groupIndex];
    Object.entries(member.paramGroupNumbers).forEach(function (nameValue) {
      params[nameValue[0]] = submatches[nameValue[1]];
    })

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
  };

  function findGroupIndex(match, nonParamGroupNumbers, nLevels) {
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

function normalizeUrl(url) {
  const q = url.indexOf('?')
  if (q === -1)
    return url.toLowerCase();
  return url.substring(0, q).toLowerCase() + url.substring(q)
}

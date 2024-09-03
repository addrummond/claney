export class Router {
  constructor(json : object, caseSensitive? : boolean)
  route(url : string) : null | {
    name: string,
    params: Record<string, string>,
    query : string,
    anchor : string,
    tags: string[],
    methods: string[]
  }
}

export function normalizeUrl(url: string) : string

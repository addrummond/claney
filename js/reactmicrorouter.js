// Based on https://github.com/ashok-khanna/react-snippets/blob/main/Router.js

let initialized = false;

// Set this to false if you want to always use pushState, even if
// window.navigation is available.
const enableNavigationApi = true;

// Simple 'bigint' counter.
function getCounter() {
  let c = [0];
  return () => {
    for (let i = 0; i < c.length; ++i) {
      if (c[i] < (1<<30)|((1 << 30)-1)) {
        for (let j = 0; j < i; ++j)
          c[j] = 0;
        ++c[i];
        return c.join(':')
      }
    }
    for (let i = 0; i < c.length; ++i)
      c[i] = 0;
    c.push(1);
    return c.join(':');
  };
}

// A globally unique identifier for a given router instance.
const getRouterId = getCounter();

// Dummy state in addition to the URL itself to force updates if the same link
// is clicked on multiple times.
const getDummyUrlState = getCounter();

function clickHandler(event) {
  const followableLink = clickHasFollowableLink(event);
  if (followableLink) {
    event.preventDefault();
    pushRoute(followableLink);
  }
}

let shouldIgnoreRoute = (r) => false;

/**
 * Specifies a function from paths to booleans determining whether or not a
 * given route should be handled by reactmicrorouter.
 *
 * @param {function} predicate
 */
export function setShouldIgnoreRoute(predicate) {
  setShouldIgnoreRoute = r => !!predicate(r);
}

let pendingNavigationResolution = null;
let setCurrentPaths = { };

function navigateHandler(event) {
  if (shouldNotInterceptNavigationEvent(event))
    return;

  const path = new URL(event.destination.url).pathname;
  if (shouldIgnoreRoute(path))
    return;

  let empty = true;
  for (const scp of Object.values(setCurrentPaths)) {
    empty = false;
    scp([getDummyUrlState, path]);
  }

  if (empty)
    return;

  event.intercept({
    handler() {
      return new Promise((resolve, _reject) => {
        if (pendingNavigationResolution)
          pendingNavigationResolutions(null);
        pendingNavigationResolution = resolve;
      });
    }
  });
}

function popstateHandler(event) {
  const path = window.location.pathname;
  if (shouldIgnoreRoute(path))
    return;

  event.preventDefault();

  for (const scp of Object.values(setCurrentPaths)) {
    scp([getDummyUrlState(), path]);
  }
}

function customEventHandler(event) {
  const path = new URL(event.detail.href).pathname;
  if (shouldIgnoreRoute(path))
    return;

  for (const scp of Object.values(setCurrentPaths)) {
    scp([getDummyUrlState(), path]);
  }
}

const customUrlChangeEventName = 'reactmicrorouter-url-change';

function initialize() {
  if (! initialized) {
    initialized = true;

    if (enableNavigationApi && window.navigation) {
      window.navigation.addEventListener('navigate', navigateHandler);
    } else {
      window.addEventListener("click", clickHandler);
      window.addEventListener("popstate", popstateHandler);
    }
    window.addEventListener(customUrlChangeEventName, customEventHandler);
  }
}

/**
 * Removes the event handlers added whenever one or more ReactMicroRouter
 * components is rendered.
 *
 * @returns {void}
 */
export function cleanup() {
  if (! initialized)
    return;
  initialized = false;

  if (enableNavigationApi && window.navigation) {
    window.navigation.removeEventListener('navigate', navigateHandler);
  } else {
    window.removeEventListener("click", clickHandler);
    window.removeEventListener("popstate", popstateHandler);
  }
  window.removeEventListener(customUrlChangeEventName, customEventHandler);
}

/**
 * A simple router component. The 'resolveRoute' property should be a function
 * from paths to rendered components. The optional 'react' property can be used
 * to pass in a React implementation (window.React is used by default).
 *
 * @param {Object} props
 * @returns {void}
 */
export function ReactMicroRouter(props) {
  const resolveRoute = props.resolveRoute;
  const react = props.react || window.React;

  const routerId = react.useRef(getRouterId());

  // state to track URL and force component to re-render on change
  const [currentPath, setCurrentPath] = react.useState([getDummyUrlState(), window.location.pathname]);

  setCurrentPaths[routerId.current] = setCurrentPath;

  react.useEffect(() => {
    initialize();

    if (pendingNavigationResolution !== null) {
      pendingNavigationResolution(null);
      pendingNavigationResolution = null;
    }
  });

  return resolveRoute(currentPath[1]);
}

/**
 * Replaces the current history entry with the specified route.
 *
 * @param {string} path
 * @returns {void}
 */
export async function replaceRoute(path) {
  await xRoute('replaceState', path);
}

/**
 * Pushes the specified route onto the history.
 *
 * @param {string} path
 * @returns {void}
 */
export async function pushRoute(path) {
  await xRoute('pushState', path);
}

async function xRoute(func, href) {
  if (enableNavigationApi && window.nagivation) {
    let args = { state: {} };
    if (func === 'replaceState')
      args.history = 'replace';
    return await window.navigation.navigate(href, args).finished;
  }

  window.history[func]({}, "", href);
  const event = new CustomEvent(customUrlChangeEventName, { detail: { href } });
  window.dispatchEvent(event);
}

function clickHasFollowableLink(event) {
  const link = event.target.closest("a");
  if (! link)
    return false;

  if (
    !event.button &&
    !event.altKey &&
    !event.ctrlKey &&
    !event.metaKey &&
    !event.shiftKey &&
    link &&
    link.href.startsWith(window.location.origin + "/") &&
    link.target !== "_blank"
  ) {
    return link.href;
  }

  return false;
}

function shouldNotInterceptNavigationEvent(event) {
  return (
    !event.canIntercept ||
    event.hashChange ||
    event.downloadRequest ||
    event.formData ||
    !event.destination.url.startsWith(window.location.origin + "/")
  );
}

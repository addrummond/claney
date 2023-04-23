// Based on https://github.com/ashok-khanna/react-snippets/blob/main/Router.js

let initialized = false;

// Set this to false if you want to always use pushState, even if
// window.navigation is available.
const enableNavigationApi = true;

// Simple 'bigint' counter.
function getCounter() {
  let c = '\x00';
  return () => {
    for (let i = 0; i < c.length; ++i) {
      if (c.charCodeAt(i) < (1 << 16)-1) {
        let newc = '';
        for (let j = 0; j < i; ++j)
          newc += '\x00';
        c = newc + String.fromCharCode(c.charCodeAt(i)+1) + c.substring(i+1);
        return c;
      }
    }
    c += '\x00';
    return c;
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

let pendingNavigationResolution = null;
let setCurrentPaths = { };

function navigateHandler(event) {
  if (shouldNotInterceptNavigationEvent(event))
    return;

  let empty = true;
  for (const scp of Object.values(setCurrentPaths)) {
    empty = false;
    scp([getDummyUrlState, new URL(event.destination.url).pathname]);
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
  for (const scp of Object.values(setCurrentPaths)) {
    scp([getDummyUrlState(), window.location.pathname]);
  }
}

function customEventHandler(event) {
  for (const scp of Object.values(setCurrentPaths)) {
    scp([getDummyUrlState(), new URL(event.detail.href).pathname]);
  }
}

function initialize() {
  if (! initialized) {
    initialized = true;

    if (enableNavigationApi && window.navigation) {
      window.navigation.addEventListener('navigate', navigateHandler);
    } else {
      window.addEventListener("click", clickHandler);
      window.addEventListener("popstate", popstateHandler);
    }
    window.addEventListener("reactmicrorouter-url-change", customEventHandler);
  }
}

export function cleanup() {
  if (! initialized)
    return;

  if (enableNavigationApi && window.navigation) {
    window.navigation.removeEventListener('navigate', navigateHandler);
  } else {
    window.removeEventListener("click", clickHandler);
    window.removeEventListener("popstate", popstateHandler);
  }
  window.removeEventListener("reactmicrorouter-url-change", customEventHandler);
}

export function ReactMicroRouter(props) {
  const resolveRoute = props.resolveRoute;
  const react = props.react || window.React;

  initialize();

  const routerId = react.useRef(getRouterId());

  // state to track URL and force component to re-render on change
  const [currentPath, setCurrentPath] = react.useState([getDummyUrlState(), window.location.pathname]);

  setCurrentPaths[routerId.current] = setCurrentPath;

  react.useEffect(() => {
    if (pendingNavigationResolution !== null) {
      pendingNavigationResolution(null);
      pendingNavigationResolution = null;
    }
  });

  return resolveRoute(currentPath[1]);
}

export async function replaceRoute(href) {
  return await xRoute('replaceState', href);
}

export async function pushRoute(href) {
  return await xRoute('pushState', href);
}

async function xRoute(func, href) {
  if (enableNavigationApi && window.nagivation) {
    let args = { state: {} };
    if (func === 'replaceState')
      args.history = 'replace';
    return await window.navigation.navigate(href, args).finished;
  }

  window.history[func]({}, "", href);
  const event = new CustomEvent('reactmicrorouter-url-change', { detail: { href } });
  window.dispatchEvent(event);
}

function clickHasFollowableLink(event) {
  const link = event.target.closest("a");
  if (! link)
    return false;

  if (link.dataset.reactMicroRouterIgnore !== undefined)
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
    // If this is just a hashChange,
    // just let the browser handle scrolling to the content.
    event.hashChange ||
    // If this is a download,
    // let the browser perform the download.
    event.downloadRequest ||
    // If this is a form submission,
    // let that go to the server.
    event.formData
  );
}

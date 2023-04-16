// Based on https://github.com/ashok-khanna/react-snippets/blob/main/Router.js

let clickListener = null;

function initEventHandler() {
  // Global Event Listener on "click"
  // Credit Chris Morgan: https://news.ycombinator.com/item?id=31373486
  if (clickListener === null) {
    clickListener = event => {
      // Only run this code when an <a> link is clicked
      const link = event.target.closest("a");
      // Correctly handle clicks to external sites and
      // modifier keys
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
        event.preventDefault();
        if (link.dataset.reactMicroRouterReplaceRoute !== undefined)
          replaceRoute(link.href);
        else
          pushRoute(link.href);
      }
    };

    window.addEventListener("click", clickListener);
  }
}

export function cleanup() {
  if (clickListener !== null)
    window.removeEventListener(clickListener);
}

export function ReactMicroRouter({ resolve, react }) {
  initEventHandler();

  // state to track URL and force component to re-render on change
  const [currentPath, setCurrentPath] = react.useState(window.location.pathname);

  react.useEffect(() => {
    const popstateHandler = () => setCurrentPath(window.location.pathname);
    const customEventHandler = (e) => setCurrentPath(new URL(e.detail.href).pathname);

    // listen for popstate event
    window.addEventListener('popstate', popstateHandler);

    // listen for custom event raised when 'navigate' is called.
    window.addEventListener('reactmicrorouter-url-change', customEventHandler);

    // clean up event listener
    return () => {
      window.removeEventListener('popstate', popstateHandler);
      window.removeEventListener('reactmicrorouter-url-change', customEventHandler);
    };
  }, [])

  return resolve(currentPath);
}

export function replaceRoute (href) {
  return xRoute('replaceState', href);
}

export function pushRoute (href) {
  return xRoute('pushState', href);
}

function xRoute(func, href) {
    // update url
    window.history[func]({}, "", href);

    // communicate to Routes that URL has changed
    const event = new CustomEvent('reactmicrorouter-url-change', { detail: { href }});
    window.dispatchEvent(event);
}

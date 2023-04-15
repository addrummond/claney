// Based on https://github.com/ashok-khanna/react-snippets/blob/main/Router.js

let clickListener = null;
let navigators = [];

function initEventHandler(navigate) {
  navigators.push(navigate);

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

        for (const n of navigators)
          n(link.href);
      }
    };

    window.addEventListener("click", clickListener);
  }
}

export function cleanup() {
  for (const n of navigators)
    window.removeEventListener(n);
}

export function getReactMicroRouter() {
  let setCurrentPath;

  const navigate = (href) => {
    if (setCurrentPath) {
      window.history.pushState({}, "", href);
      setCurrentPath(new URL(href).pathname);
    }
  }

  initEventHandler(navigate);

  return {
    ReactMicroRouter(props) {
      return Router(props, f => setCurrentPath = f);
    },
    navigate
  }
}

function Router({ resolve, react }, assignSetCurrentPath) {
  // state to track URL and force component to re-render on change
  const [currentPath, setCurrentPath] = react.useState(window.location.pathname);
  assignSetCurrentPath(setCurrentPath);

  react.useEffect(() => {
    const onLocationChange = () => {
      // update path state to current window URL
      setCurrentPath(window.location.pathname);
    }

    // listen for popstate event
    window.addEventListener('popstate', () => onLocationChange());

    // clean up event listener
    return () => {
      window.removeEventListener('popstate', onLocationChange)
    };
  }, [])

  return resolve(currentPath);
}

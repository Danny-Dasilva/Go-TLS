const initCycleTLS = require("../dist/index.js");

(async () => {
  // Initiate cycleTLS
  const cycleTLS = await initCycleTLS();
  const complexCookies = [
    {
      name: "cookie1",
      value: "value1",
      domain: "httpbin.org",
    },
    {
      name: "cookie2",
      value: "value2",
      domain: "httpbin.org",
    },
  ];

  const response = await cycleTLS("https://httpbin.org/cookies", {
    cookies: complexCookies,
  });

  const result = await response.json();
  console.log(result);
  /* Expected
  {
    "cookies": {
      "cookie1": "value1",
      "cookie2": "value2"
    }
  }
  */
  await cycleTLS.exit();
})();

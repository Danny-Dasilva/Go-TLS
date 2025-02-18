import initCycleTLS, { CycleTLSClient } from "../dist/index.js";

describe("CycleTLS InsecureSkipVerify Test", () => {
  let cycleTLS: CycleTLSClient;
  let ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
  let userAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9125 });
  });

  afterAll(async () => {
    await cycleTLS.exit();
  });

  test("Should return a handshake error for insecureSkipVerify", async () => {
    const url = "https://expired.badssl.com";
    const response = await cycleTLS(
      url,
      {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        insecureSkipVerify: false,
      },
      "get"
    );

    expect(await response.text()).toContain(
      "uTlsConn.Handshake() error: tls: failed to verify certificate: x509: certificate has expired or is not yet valid"
    );
  });

  test("Should return a 200 response for insecureSkipVerify", async () => {
    const url = "https://expired.badssl.com";
    const response = await cycleTLS(
      url,
      {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        insecureSkipVerify: true,
      },
      "get"
    );

    expect(response.status).toBe(200);
  });
});

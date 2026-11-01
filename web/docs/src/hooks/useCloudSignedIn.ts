import { useEffect, useState } from "react";

// Marmot Cloud sets this cookie on marmotdata.io while it holds a live session. It carries no token, only a hint, so the site can offer the console instead of sign-up; the console still checks the real session
const HINT = /(?:^|;\s*)marmot_signed_in=1(?:;|$)/;

export const CONSOLE_URL = "https://cloud.marmotdata.io/console";

export function useCloudSignedIn(): boolean {
  const [signedIn, setSignedIn] = useState(false);
  useEffect(() => {
    setSignedIn(HINT.test(document.cookie));
  }, []);
  return signedIn;
}

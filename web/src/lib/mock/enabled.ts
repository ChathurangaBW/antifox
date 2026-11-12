// Mock Switches. Open variable injected during build period (NEXT_PUBLIC_ The prefix is readable in the browser.
// Vercel Set Up NEXT_PUBLIC_MOCK=1 That's right. mock、No backend required.
export const MOCK = process.env.NEXT_PUBLIC_MOCK === "1";

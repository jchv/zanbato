const HEX_CHARS = "0123456789ABCDEF";

/** Format a single byte as a two-character uppercase hex string. */
export function byteToHex(b: number): string {
  return HEX_CHARS[(b >> 4) & 0xf]! + HEX_CHARS[b & 0xf]!;
}

/**
 * Format an offset as a left-padded uppercase hex string.
 */
export function formatOffset(offset: number, width = 8): string {
  return offset.toString(16).padStart(width, "0").toUpperCase();
}

/**
 * Format a byte for the ASCII view. Non-printable characters are rendered as
 * '.', as is tradition.
 */
export function byteToAscii(b: number): string {
  if (b >= 0x20 && b <= 0x7e) {
    return String.fromCharCode(b);
  }
  return ".";
}

/**
 * Compute a reasonable offset width for the offset column based on file size.
 * This always chooses a multiple of two that is greater than or equal to 4.
 */
export function offsetWidthFor(length: number): number {
  if (length <= 0) return 4;
  const hex = (length - 1).toString(16);
  return Math.max(4, hex.length + (hex.length % 2));
}

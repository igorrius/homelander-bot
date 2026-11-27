/** Apartment numbers of a house as Select options, from..to inclusive. */
export function apartmentOptions(house: { apartmentFrom: number; apartmentTo: number } | undefined): string[] {
  if (!house) return []
  const out: string[] = []
  for (let n = house.apartmentFrom; n <= house.apartmentTo; n++) out.push(String(n))
  return out
}

import { modules as product, type ModuleEntry } from '../../../src/lib/modules';
import BrightImage from './BrightImage.svelte';
import Card from './Card.svelte';
import Fits from './Fits.svelte';
import GradientFill from './GradientFill.svelte';
import GradientScrim from './GradientScrim.svelte';
import Grouped from './Grouped.svelte';
import LitPanel from './LitPanel.svelte';
import ModernColour from './ModernColour.svelte';
import Outlined from './Outlined.svelte';
import Overflows from './Overflows.svelte';
import RegionFill from './RegionFill.svelte';
import Scrim from './Scrim.svelte';
import Throws from './Throws.svelte';
import TypeScale from './TypeScale.svelte';
import Unavailable from './Unavailable.svelte';

/** The display's own entries are spread last, so a stub cannot take a module's name. */
export const modules: Record<string, ModuleEntry> = {
  fits: { component: Fits },
  overflows: { component: Overflows },
  'type-scale': { component: TypeScale },
  grouped: { component: Grouped },
  'bright-image': { component: BrightImage },
  unavailable: { component: Unavailable },
  throws: { component: Throws },

  // Seeds. A fixture placing one of these is asserting that the emission scan reports it.
  'lit-panel': { component: LitPanel },
  card: { component: Card },
  'region-fill': { component: RegionFill },
  outlined: { component: Outlined },
  scrim: { component: Scrim },
  'gradient-fill': { component: GradientFill },
  'gradient-scrim': { component: GradientScrim },
  'modern-colour': { component: ModernColour },

  ...product,
};

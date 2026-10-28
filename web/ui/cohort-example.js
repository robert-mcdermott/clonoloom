// Guided artificial cohorts use ordinary actions and immutable analysis parents. No
// scientific calculation happens here; preset annotations mirror the generator truth.
export async function cohortExampleStep(app, actions, dataset, { preset, step }) {
  if (!['counts-independent', 'counts-paired', 'counts-null', 'counts-unbalanced', 'counts-confounded'].includes(preset)) throw new Error('Choose a documented cohort example preset.');
  if (!['design', 'features', 'compare', 'counts'].includes(step)) throw new Error('Choose design, features, compare or counts.');
  const same = app.store.sameProject();
  const opened = await app.ensureDataset(dataset);
  if (!same()) throw new Error('The project changed.');
  const example = (await app.examples.refresh())?.find((e) => e.name === `simulated-${preset}`);
  const normalized = (path) => String(path).replaceAll('\\', '/');
  if (!same()) throw new Error('The project changed.');
  if (opened.status !== 'open' || !example?.path || !dataset.sources.length || dataset.sources.some((s) => !normalized(s.path).startsWith(normalized(example.path) + '/'))) throw new Error('Open the matching artificial example before using its prescribed steps.');
  const paired = preset === 'counts-paired', confounded = preset === 'counts-confounded';
  const expected = {};
  for (let i = 0; i < (paired ? 28 : 32); i++) for (let j = 0; j < (paired ? 2 : 1); j++) {
    expected[`S${String(i + 1).padStart(2, '0')}-${j + 1}`] = {
      subject: `S${String(i + 1).padStart(2, '0')}`, batch: `B${confounded ? (i < 16 ? 1 : 2) : (i + j) % 2 + 1}`,
      condition: paired ? 'paired stimulation' : i < (preset === 'counts-unbalanced' ? 12 : 16) ? 'control' : 'stimulated',
      ...(paired ? { timepoint: j ? 'day 14' : 'day 0' } : {}),
    };
  }
  if (opened.summary.samples.length !== Object.keys(expected).length || opened.summary.samples.some((s) => !Object.hasOwn(expected, s.name))) throw new Error('Example sample membership differs from the prescribed study.');
  const name = `Guided ${preset}`;
  if (step === 'design') {
    await actions.annotate_samples({dataset:dataset.id, samples:expected});
    if (!same()) throw new Error('The project changed.');
    return actions.check_cohort_design({dataset:dataset.id, name, save:true, spec:{unit:'subject',layout:paired?'paired':'independent',groupField:paired?'timepoint':'condition',levels:paired?['day 0','day 14']:['control','stimulated'],exclude:{}}});
  }
  if (confounded) throw new Error('The confounded study has no identifiable group contrast. Keep and inspect its failed design check.');
  const design = [...(app.store.project.cohortDesigns ?? [])].reverse().find((d) => d.dataset === dataset.id && d.name === name);
  if (!design?.result.ok) throw new Error('Complete the guided design step first.');
  if (step === 'features') {
    const measured = await actions.measure_diversity({dataset:dataset.id});
    if (!same()) throw new Error('The project changed.');
    return actions.build_cohort_features({design:design.id, diversity:measured.data.result});
  }
  // Resolve by the exact design version, never by whichever result is currently visible.
  const candidates = app.store.project.artifacts.filter((a) => a.dataset === dataset.id && a.kind === 'cohort-features').reverse();
  let features;
  for (const a of candidates) {
    const response = await fetch(`api/cohort-features/${encodeURIComponent(a.id)}`);
    if (!response.ok) throw new Error('The saved feature evidence is unavailable.');
    const value = await response.json();
    if (!same()) throw new Error('The project changed.');
    if (value.result.selection.record === design.id) { features = a.id; break; }
  }
  if (!features) throw new Error('Save the guided feature table for this design version first.');
  return step === 'compare'
    ? actions.compare_cohort({features,options:{features:['richness','shannon','hill1'],confidence:.95,permutations:9999,seed:20261025}})
    : actions.fit_cohort_counts({features,options:{minCount:10,minSamples:12,adjustBatch:true,confidence:.95}});
}

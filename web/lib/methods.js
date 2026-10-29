// A methods paragraph for a dataset (clonoloom-spec/requirements.md, O2), written from what the
// project records of it (its files, format, unit, validation, filter and clonotype definition),
// what the program computed (the rows its filter kept, its clonotypes) and ClonoLoom's version,
// with the references it cites, numbered in the order they are first cited. Only what was done is
// said: nothing is written for a step that did not happen. Deterministic: the same inputs give
// the same text. Adapted from SituCraft 0.9.0 web/lib/methods.js.

import { UNITS, datasetUnit } from './datasets.js';
import { describeRules } from './filters.js';
import { choiceWords } from './tracking.js';

const count = (n) => Number(n).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;

export function joinList(parts) {
  if (parts.length <= 1) return parts.join('');
  return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;
}

// Expression fits have no receptor dataset of their own. Include a saved fit only through
// an explicit input binding or one of this dataset's saved phenotype/graph/export results.
export function cellResultsForDataset(project, dataset) {
  const cells = project.artifacts.filter((r) => r.dataset === dataset && ['phenotypes', 'modularity', 'conga', 'anndata-export'].includes(r.kind));
  const inputs = new Set([...(project.expression ?? []).filter((e) => e.dataset === dataset).map((e) => e.id), ...cells.map((r) => r.expression)]);
  const maps = project.artifacts.filter((r) => r.kind === 'expression-map' && inputs.has(r.expression));
  return [...maps, ...cells];
}

// The works a paragraph cites.
export const REFERENCES = {
  trust4: 'Song L, Cohen D, Ouyang Z, et al. TRUST4: immune repertoire reconstruction from bulk and single-cell RNA-seq data. Nat Methods. 2021;18:627–630. doi:10.1038/s41592-021-01142-2',
  vdjtools: 'Shugay M, Bagaev DV, Turchaninova MA, et al. VDJtools: Unifying Post-analysis of T Cell Receptor Repertoires. PLoS Comput Biol. 2015;11:e1004503. doi:10.1371/journal.pcbi.1004503',
  conga: 'Schattgen SA, Guion K, Crawford JC, et al. Integrating T cell receptor sequences and transcriptional profiles by clonotype neighbor graph analysis (CoNGA). Nat Biotechnol. 2022;40:54–63. doi:10.1038/s41587-021-00989-2',
  gem: 'Van Rhijn I, et al. Discovery of invariant T cells by next-generation sequencing of the human TCR alpha-chain repertoire. J Immunol. 2014;193:5338–5344. doi:10.4049/jimmunol.1401380',
  airr: 'Vander Heiden JA, Marquez S, Marthandan N, et al. AIRR Community standardized representations for annotated immune repertoires. Front Immunol. 2018;9:2206. doi:10.3389/fimmu.2018.02206',
  miairr: 'Rubelt F, Busse CE, Bukhari SAC, et al. Adaptive Immune Receptor Repertoire Community recommendations for sharing immune-repertoire sequencing data. Nat Immunol. 2017;18:1274–1278. doi:10.1038/ni.3873',
  imgtInput: 'Brochet X, Lefranc MP, Giudicelli V. IMGT/V-QUEST: the highly customized and integrated system for IG and TR standardized V-J and V-D-J sequence analysis. Nucleic Acids Res. 2008;36:W503–508. doi:10.1093/nar/gkn316',
  imgtNames: 'Lefranc MP. Immunoglobulin and T cell receptor genes: IMGT and the birth and rise of immunoinformatics. Front Immunol. 2014;5:22. doi:10.3389/fimmu.2014.00022',
  imgtJunction: 'Lefranc MP, Pommié C, Ruiz M, et al. IMGT unique numbering for immunoglobulin and T cell receptor variable domains and Ig superfamily V-like domains. Dev Comp Immunol. 2003;27:55–77. doi:10.1016/S0145-305X(02)00039-3',
  hgnc: 'Seal RL, Braschi B, Gray K, et al. Genenames.org: the HGNC resources in 2023. Nucleic Acids Res. 2023;51:D1003–D1009. doi:10.1093/nar/gkac888',
  ensembl: 'Dyer SC, Austine-Orimoloye O, Azov AG, et al. Ensembl 2025. Nucleic Acids Res. 2025;53:D948–D957. doi:10.1093/nar/gkae1071',
  selection: 'Elhanati Y, Murugan A, Callan CG Jr, Mora T, Walczak AM. Quantifying selection in immune receptor repertoires. Proc Natl Acad Sci USA. 2014;111:9875–9880. doi:10.1073/pnas.1409572111',
  hill: 'Hill MO. Diversity and evenness: a unifying notation and its consequences. Ecology. 1973;54:427–432. doi:10.2307/1934352',
  chao1984: 'Chao A. Nonparametric estimation of the number of classes in a population. Scand J Stat. 1984;11:265–270.',
  chaoLee: 'Chao A, Lee SM. Estimating the number of classes via sample coverage. J Am Stat Assoc. 1992;87:210–217. doi:10.1080/01621459.1992.10475194',
  chaoWangJost: 'Chao A, Wang YT, Jost L. Entropy and the species accumulation curve: a novel entropy estimator via discovery rates of new species. Methods Ecol Evol. 2013;4:1091–1100. doi:10.1111/2041-210X.12108',
  chao2014: 'Chao A, Gotelli NJ, Hsieh TC, Sander EL, Ma KH, Colwell RK, Ellison AM. Rarefaction and extrapolation with Hill numbers: a framework for sampling and estimation in species diversity studies. Ecol Monogr. 2014;84:45–67. doi:10.1890/13-0133.1',
  jaccard: 'Jaccard P. The distribution of the flora in the alpine zone. New Phytol. 1912;11:37–50. doi:10.1111/j.1469-8137.1912.tb05611.x',
  dice: 'Dice LR. Measures of the amount of ecologic association between species. Ecology. 1945;26:297–302. doi:10.2307/1932409',
  morisita: 'Morisita M. Measuring of interspecific association and similarity between communities. Mem Fac Sci Kyushu Univ Ser E Biol. 1959;3:65–80',
  horn: 'Horn HS. Measurement of "overlap" in comparative ecological studies. Am Nat. 1966;100:419–424. doi:10.1086/282436',
  horvitzThompson: 'Horvitz DG, Thompson DJ. A generalization of sampling without replacement from a finite universe. J Am Stat Assoc. 1952;47:663–685. doi:10.1080/01621459.1952.10483446',
  dash: 'Dash P, Fiore-Gartland AJ, Hertz T, et al. Quantifiable predictive features define epitope-specific T cell receptor repertoires. Nature. 2017;547:89–93. doi:10.1038/nature22383',
  oas: 'Olsen TH, Boyles F, Deane CM. Observed Antibody Space: a diverse database of cleaned, annotated, and translated unpaired and paired antibody sequences. Protein Sci. 2022;31:141–146. doi:10.1002/pro.4205',
  ots: 'Raybould MIJ, Greenshields-Watson A, Agarwal P, et al. The Observed T cell receptor Space database enables paired-chain repertoire mining, coherence analysis, and language modeling. Cell Rep. 2024;43:114704. doi:10.1016/j.celrep.2024.114704',
  tcrdist3: 'Mayer-Blackwell K, Schattgen S, Cohen-Lavi L, et al. TCR meta-clonotypes for biomarker discovery with tcrdist3 enabled identification of public, HLA-restricted clusters of SARS-CoV-2 TCRs. eLife. 2021;10:e68605. doi:10.7554/eLife.68605',
  chaoJost: 'Chao A, Jost L. Coverage-based rarefaction and extrapolation: standardizing samples by completeness rather than size. Ecology. 2012;93:2533–2547. doi:10.1890/11-1952.1',
  rytlewski: 'Rytlewski J, Deng S, Xie T, et al. Model to improve specificity for identification of clinically-relevant expanded T cells in peripheral blood. PLoS One. 2019;14:e0213684. doi:10.1371/journal.pone.0213684',
  dewitt: 'DeWitt WS, Emerson RO, Lindau P, et al. Dynamics of the cytotoxic T cell response to a model of acute viral infection. J Virol. 2015;89:4517–4526. doi:10.1128/JVI.03474-14',
  bh: 'Benjamini Y, Hochberg Y. Controlling the false discovery rate: a practical and powerful approach to multiple testing. J R Stat Soc B. 1995;57:289–300. doi:10.1111/j.2517-6161.1995.tb02031.x',
  storey: 'Storey JD. A direct approach to false discovery rates. J R Stat Soc B. 2002;64:479–498. doi:10.1111/1467-9868.00346',
  wilson: 'Wilson EB. Probable inference, the law of succession, and statistical inference. J Am Stat Assoc. 1927;22:209–212. doi:10.1080/01621459.1927.10502953',
  sethna: 'Sethna Z, Elhanati Y, Callan CG Jr, Walczak AM, Mora T. OLGA: fast computation of generation probabilities of B- and T-cell receptor amino acid sequences and motifs. Bioinformatics. 2019;35:2974–2981. doi:10.1093/bioinformatics/btz035',
  marcou: 'Marcou Q, Mora T, Walczak AM. High-throughput immune repertoire analysis with IGoR. Nat Commun. 2018;9:561. doi:10.1038/s41467-018-02832-w',
};

const coverageWords = (v) => (v > 0.99 && v < 1 ? v.toFixed(4) : v.toFixed(3));

const UNIT_WORDS = { cells: 'cells', templates: 'templates', reads: 'reads', umis: 'UMIs', sequences: 'sequences' };

// The formats' names, as a methods section says them.
const FORMATS = {
  airr: 'AIRR Rearrangement TSV',
  'imgt-highvquest': 'IMGT/HighV-QUEST tables',
  'cellranger-contigs': '10x Genomics Cell Ranger contig annotations',
  'immunoseq-v3': 'Adaptive Biotechnologies immunoSEQ exports (v3 columns)',
  'immunoseq-v2': 'Adaptive Biotechnologies immunoSEQ exports (v2 columns)',
  'mixcr-clones': 'MiXCR clone tables',
  'oas-unit': 'Observed Antibody Space or Observed T cell receptor Space data units',
  'tcrdist3-clones': 'tcrdist3 clone tables',
  'trust4-report': 'TRUST4 repertoire reports',
  vdjtools: 'VDJtools clonotype tables',
  table: 'a table with mapped columns',
};

// The fields a mapping names, as a methods section says them (internal/reads/mapped.go).
const MAPPED_FIELDS = {
  junction_aa: 'the junction\'s amino acids', junction: 'the junction\'s nucleotides', v_call: 'the V gene', d_call: 'the D gene', j_call: 'the J gene', c_call: 'the C gene',
  count: 'the count', sample: 'the sample', locus: 'the locus', productive: 'productivity', cell_id: 'the cell', clone_id: 'the clone', sequence_id: 'the sequence\'s ID',
};
const order = (field) => {
  const i = Object.keys(MAPPED_FIELDS).indexOf(field);
  return i < 0 ? Infinity : i;
};

const day = (iso) => String(iso ?? '').slice(0, 10);

// The paragraph and its references. inputs: { dataset (the project's record), summary (the
// program's summary of the open dataset, or null), filter (the filter version its analyses use,
// or null) and filterResult ({ rows, kept } the program counted, or null), definition (its
// clonotype definition) and clonotypes ({ id, describe, count, cells } from the program, or
// null), results (the project's records of the results kept of it: lib/results.js), program:
// { version, commit } }.
export function methodsParagraph({ dataset, summary = null, filter = null, filterResult = null, definition = null, clonotypes = null, results = [], cohortDesigns = [], program }) {
  const refs = [];
  const cite = (key, text = REFERENCES[key]) => {
    let i = refs.findIndex((r) => r.key === key);
    if (i < 0) {
      refs.push({ key, text });
      i = refs.length - 1;
    }
    return `[${i + 1}]`;
  };
  const sentences = [];
  for (const record of cohortDesigns.filter((d) => d.dataset === dataset.id)) {
    const check = record.result;
    sentences.push(`Cohort design ${record.name}, version ${record.revision} (${check.id}): ${check.ok ? 'structural checks passed' : 'FAILED CHECK; unavailable for inference'}. ${check.words.join(' ')}`);
    for (const finding of check.findings ?? []) sentences.push(`${finding.code}: ${finding.detail}`);
    for (const source of check.citations ?? []) sentences.push(`Design reference ${cite(`design:${source}`, source)}.`);
  }
  const sources = dataset.sources ?? [];
  const files = sources.length === 1 ? 'The file' : `The ${count(sources.length)} files`;

  // Where the data come from: public data with their license and citation.
  for (const c of dataset.credits ?? []) {
    const terms = c.licenseClass === 'open' ? `published under ${c.license}` : `used under ${c.license}`;
    sentences.push(`The data are ${c.title}, ${terms} ${cite(`data:${c.example}`, c.citation)}${dataset.opened ? `, downloaded and first read on ${day(dataset.opened)}` : ''}.`);
  }

  // Simulated data (clonoloom simulate): how they were made.
  const simulations = (summary?.samples ?? []).map((s) => s.repertoire).filter((r) => r?.studyId?.startsWith('clonoloom-simulated-'));
  const bcellModels = ['clonoloom-simulated-clonoloom-artificial-bcell-v1', 'clonoloom-simulated-clonoloom-artificial-bcell-v2'];
  const geneModels = ['clonoloom-simulated-pseudobulk-de-study', 'clonoloom-simulated-pseudobulk-de-null'];
  if(simulations.some((r)=>geneModels.includes(r.studyId)))sentences.push('The artificial paired pseudobulk expression data use one baseline per subject (28 subjects in the presets), 48–52 singleton clones and eight larger clones per subject, fictional paired TRA/TRB receptors, and 40 artificial genes. Shared subject/gene Gamma(16,1)/16 and category/gene Gamma(5,1)/5 rates precede conditionally Poisson cell counts. Positive controls have threefold increases or decreases; the null preset has no generating category effect. Low-count, zero-gene and zero-subject controls remain explicit. Supplied correlated Gaussian coordinates are illustrative, not an expression embedding. Full assignments, generating rates, pseudobulk sums and source hashes are separate truth. This is a constructed benchmark, not fitted biology, antigen specificity or evidence that cells independently replicate subjects.');
  const cellModels = ['clonoloom-simulated-cell-states', 'clonoloom-simulated-cell-states-clouds', 'clonoloom-simulated-pseudobulk-study'];
  const batchQCModels=['clonoloom-simulated-batch-qc-study-v1'];
  if(simulations.some((r)=>batchQCModels.includes(r.studyId)))sentences.push('The artificial batch-QC study contains 26 samples from 25 fictional subjects across four declared batches and explicit repeat/missing-batch exclusions. Length-selected human TRB junctions, deliberate stop codons and missing annotations accompany uneven shifted gamma-Poisson counts. A shifted batch, low-productivity observation, missing count, small group and constant row counts are construction controls, not calibrated biological effects or failure diagnoses.');
  const contaminationModels=['clonoloom-simulated-contamination-study-v1'];
  if(simulations.some((r)=>contaminationModels.includes(r.studyId)))sentences.push('The artificial contamination-evidence study has eight fictional subjects and nine samples, with a repeat, uneven shifted gamma-Poisson UMI counts, three planted transfers, independently shared controls and a synonymous control. Productive sequences follow ClonoLoom’s length-selected human TRB simulation model. Sharing labels are generating truth, not Pgen labels or proof of contamination; no empirical index-hopping rate or classifier is fitted.');
  const convergenceModels = ['clonoloom-simulated-convergence-study-v1'];
  if (simulations.some((r) => convergenceModels.includes(r.studyId))) sentences.push('The artificial convergence/HLA study has 32 fictional subjects and 36 samples, including four repeated observations. Varied junctions and uneven shifted gamma-Poisson UMI counts accompany planted QGHT recurrence and a conservative ILMV control. An association with HLA-A*02:01 and four unresolved A typings are construction choices, not fitted haplotypes, biological restriction or antigen evidence. Complete subject, typing, arm and receptor truth is retained.');
  const motifModels = ['clonoloom-simulated-motif-study-v1'];
  if (simulations.some((r) => motifModels.includes(r.studyId))) sentences.push('The artificial motif study contains four query and eight background subjects, variable junction lengths, biased central residues and arbitrary TRBV labels. Planted local QGHT and conservative global ILMV signatures are known construction choices, not antigen annotations. Uneven shifted gamma-Poisson UMI counts are descriptive, not statistical weights. Complete sequence, arm and signature truth is saved; this is not a fitted recombination model.');
  const definitionModels = ['clonoloom-simulated-definition-variants-v1'];
  if (simulations.some((r) => definitionModels.includes(r.studyId))) sentences.push('The definition-sensitivity study is artificial: long-tailed gamma-Poisson UMI counts, uneven sampling exposure and explicit synonymous junction/V-allele distinctions create different receptor partitions. Comparison subjects have triples of nucleotide junctions sharing an amino-acid junction and different V/allele calls, with a 1.6-fold private-count scale. The construction and observed depth are deliberately definition-sensitive; it is not a fitted biological population or a depth-standardized comparison. Complete generating and partition truth is retained.');
  const countModels = ['clonoloom-simulated-replicated-counts-v1', 'clonoloom-simulated-counts-unbalanced', 'clonoloom-simulated-counts-confounded'];
  const trbModels = ['standard', 'null', 'small', 'car-t', 'transplant', 'vaccine'].map((name) => `clonoloom-simulated-${name}`);
  const bcellSimulation = simulations.some((r) => bcellModels.includes(r.studyId));
  const cellSimulations = simulations.filter((r) => cellModels.includes(r.studyId));
  const simulated = simulations.find((r) => trbModels.includes(r.studyId));
  const unknown = [...new Set(simulations.filter((r) => ![...geneModels, ...bcellModels, ...cellModels, ...definitionModels, ...batchQCModels, ...contaminationModels, ...convergenceModels, ...motifModels, ...countModels, ...trbModels].includes(r.studyId)).map((r) => r.studyId))];
  if (simulations.some((r) => countModels.includes(r.studyId))) sentences.push('The replicated repertoire counts were simulated with heterogeneous gamma-Poisson variation, lognormal sampling exposures, subject effects, two batch levels, 48 shared fictional receptors and a private long-tailed background. Sparse increases/decreases or an all-null group contrast are recorded in the saved parameters and truth. Fictional stop-free junctions and SIM calls do not represent fitted human recombination or antigen specificity. Generator exposures differ from realized library totals; fitted total-count-offset effects describe relative repertoire abundance.');
  if (unknown.length) sentences.push(`The data include simulated studies ${joinList(unknown)}. These identifiers do not establish their generative model; consult each study's saved parameters and truth before describing how it was simulated.`);
  if (cellSimulations.length) {
    sentences.push(`The paired TRA/TRB and expression data were made by ClonoLoom's artificial cell-state simulator ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)} (studies ${[...new Set(cellSimulations.map((r) => r.studyId.replace('clonoloom-simulated-', '')))].join(', ')}). Clones have fictional SIM gene calls and independently drawn junctions; cells draw clones uniformly (cell-state presets) or with probability proportional to inverse rank (pseudobulk-study), and one of three states by the recorded sample-specific rule. The 24 artificial genes have conditionally independent negative-binomial counts (size 4; mean 16 in the state's eight-gene block, 2 otherwise). Supplied grid or Gaussian-cloud coordinates illustrate the known state, independently of the expression fit; they are not inferred embeddings. Forced missing evidence and an all-zero cell test bookkeeping. Separate truth records the model, assignments and source hashes. This controlled benchmark is not fitted biology, does not simulate antigen specificity, and does not make cells independent biological replicates.`);
  }
  if (bcellSimulation) {
    sentences.push(`The artificial paired B-cell sequences were made by ClonoLoom's simulator ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)} (models ${[...new Set(simulations.filter((r) => bcellModels.includes(r.studyId)).map((r) => r.studyId.replace('clonoloom-simulated-', '')))].join(', ')}): fictional SIM gene calls, coding sequences with uniform proposed substitutions restricted to preserve junction anchors and avoid stop codons, a fixed seven-node family tree, deliberate back-mutations, and fixed forward isotype transitions. B cells were sampled independently at days -7, 14 and 28 of a simulated immunization from recorded family and node probabilities. Ancestors, ordered mutation events, isotypes and observed cell assignments are recorded in truth/. Supplied ancestor alignments and region coordinates are artificial; this is not a fitted SHM or affinity model. No read counts, UMI counts, sequencing errors or additional biological variance were simulated for these B cells.`);
  }
  if (simulated) {
    sentences.push(`${bcellSimulation ? 'The T-cell receptor β data' : 'The data'} are simulated (${simulated.studyTitle || simulated.studyId}): T-cell receptor β rearrangements made by ClonoLoom's simulator ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)} from GRCh38's gene segments at Ensembl's annotation ${cite('ensembl')}, with the V and J usage of 10x Genomics' 10k PBMC data, deletions and non-templated insertions at the junctions, and selection on junction length to that data's ${cite('selection')}; each subject's clones of power-law sizes, each sample templates drawn from them and sequenced into reads. Their truth is recorded beside them.`);
    // A study over time: its samples' time points and reference samples say so.
    const repertoires = simulations.filter((r) => trbModels.includes(r.studyId));
    const event = repertoires.map((r) => /\(([^()]+)\)$/.exec(r.timePoint ?? '')?.[1]).find(Boolean);
    if (event) {
      const tissues = repertoires.map((r) => r.tissue ?? '');
      const reference = tissues.includes('infusion product')
        ? ' An infusion product, made from each subject\'s own T cells and expanded ex vivo, was mixed into its blood in known shares after the infusion, and sampled as its reference sample.'
        : tissues.includes('donor blood')
          ? ' A donor\'s repertoire was mixed into each recipient\'s blood in known shares after the transplant, and sampled as its reference sample.'
          : '';
      sentences.push(`Each subject was sampled at several time points, in days from the ${event}, its clones' frequencies following known trajectories and varying between samples beyond sampling, as between blood samples ${cite('rytlewski')}.${reference}`);
    }
  }

  // How they were read: the formats by the readers' codes, in the order of the files.
  const codes = [...new Set(sources.map((s) => s.format))];
  const formats = codes.length ? codes.map((f) => FORMATS[f] ?? f) : (dataset.formats ?? []);
  const program_ = `ClonoLoom ${program.version}${program.commit ? ` (commit ${program.commit.slice(0, 7)})` : ''} ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}`;
  sentences.push(`${files} (${joinList(formats)}) ${sources.length === 1 ? 'was' : 'were'} read with ${program_} by its own readers (reader version ${dataset.readerVersion}), which recorded ${sources.length === 1 ? 'its' : 'each file\'s'} SHA-256 checksum as ${sources.length === 1 ? 'it' : 'each'} was read.`);
  if (codes.includes('airr')) sentences.push(`AIRR files were read as the AIRR Community's Rearrangement schema defines them ${cite('airr')}.`);
  if (codes.includes('airr') && (summary?.readerVersion ?? dataset.readerVersion ?? 1) >= 2) sentences.push('Supplied sequence, alignment, orientation and coordinate evidence was retained without reconstruction. Structural checks followed AIRR Rearrangement schema 2.0; missing evidence was not inferred and invalid annotations were preserved for inspection.');
  if (codes.includes('mixcr-clones')) sentences.push('MiXCR clone tables were read with each segment\'s best-scoring hits as its call, MiXCR\'s allele *00 (no allele resolved) dropped, and MiXCR\'s CDR3 (from the conserved cysteine to the phenylalanine or tryptophan, both included) as the junction.');
  if (codes.includes('trust4-report')) sentences.push(`TRUST4 reports ${cite('trust4')} were read with source junction anchors retained, underscore stops represented as *, ambiguous residues as X and partial junctions retained only as native evidence. Modern count columns did not establish reads versus barcode support; an explicit legacy read_count column stated reads. No cell identities or pairing were inferred.`);
  if (codes.includes('vdjtools')) sentences.push(`VDJtools tables ${cite('vdjtools')} were read with source junction anchors retained; underscore or tilde incomplete-codon markers did not become continuous amino-acid junctions. Counts did not establish reads versus molecules. Alternative gene calls remained sets, and segment-boundary extensions remained native evidence.`);
  if (codes.includes('trust4-report') || codes.includes('vdjtools')) sentences.push('Original native header and row text, including exact numeric tokens and unsupported annotations, was retained for inspection. Source frequencies were not renormalized or used to reconstruct counts; separate IG/TR or pre-subset denominators may apply. Missing peptides and counts were not imputed. Junction-level frame/stop flags do not certify full-transcript function or biological validity.');
  if (codes.includes('oas-unit')) {
    // The database the units come from, as their loci say: OAS holds antibodies, OTS T-cell receptors.
    const loci = (summary?.samples ?? []).flatMap((x) => Object.keys(x.loci ?? {}));
    const t = loci.some((l) => l.startsWith('TR'));
    const b = loci.some((l) => l.startsWith('IG'));
    const oas = () => `the Observed Antibody Space ${cite('oas')}`;
    const ots = () => `the Observed T cell receptor Space ${cite('ots')}`;
    const from = t && !b ? ots() : b && !t ? oas() : `${oas()} and ${ots()}`;
    sentences.push(`Data units of ${from} were read with their metadata as their samples'; a paired unit's rows as single cells, each with its two chains, named by their contigs' barcode.`);
  }
  if (codes.includes('imgt-highvquest')) sentences.push(`IMGT tables ${cite('imgtInput')} were joined by sequence number and sequence ID, counting annotated sequences without inferring reads or molecules. Oriented query sequences and IMGT gaps were retained. Query coordinates were mapped only when their region matched the Summary sequence without reported indel correction; native annotations and contributing file fingerprints were preserved. No germline alignment was reconstructed.`);
  if (codes.includes('tcrdist3-clones')) sentences.push(`tcrdist3's clone tables ${cite('tcrdist3')} were read with each clone of n cells as n cells with its chains, a sample for each subject and epitope.`);
  // Tables of other tools, read through the columns the user mapped.
  for (const s of sources.filter((x) => x.mapping)) {
    const m = s.mapping;
    const fields = Object.keys(m.columns ?? {}).sort((a, b) => order(a) - order(b));
    const columns = fields.map((field) => `${MAPPED_FIELDS[field] ?? field} from "${m.columns[field]}"`);
    const counts = m.columns?.count ? `, the count being of ${UNIT_WORDS[m.unit] ?? m.unit}` : ', each row one sequence (no count was mapped)';
    const inferred = m.columns?.productive ? '' : '; a rearrangement was taken as productive when its junction is whole codons with no stop codon';
    sentences.push(`${s.name}'s columns were mapped by the user: ${joinList(columns)}${counts}${m.sample ? `, every row of sample ${m.sample}` : ''}${inferred}.`);
  }
  if (summary?.samples?.some((s) => s.repertoire)) {
    const matched = summary.samples.filter((s) => s.repertoire).length;
    sentences.push(`AIRR Repertoire metadata ${cite('miairr')} were matched to ${matched === summary.samples.length ? 'every sample' : `${plural(matched, 'sample')} of ${count(summary.samples.length)}`} by repertoire_id.`);
  }

  // What they hold, and in which unit.
  const unit = datasetUnit(dataset);
  const unitText = unit
    ? `counted in ${UNITS[unit] ?? unit}${dataset.unitConfirmed ? ' (the unit confirmed by the user, which the files did not state)' : ''}`
    : codes.includes('trust4-report') ? 'whose count units remain unspecified, including possible barcode support without cell identities' : 'whose counts the files do not say are of reads or of molecules';
  sentences.push(`The dataset holds ${plural(dataset.samples.length, 'sample')} and ${plural(dataset.rows, dataset.singleCell ? 'contig' : 'rearrangement')}${dataset.singleCell ? ' of single cells' : ''}, ${unitText}.`);

  // Gene names.
  const mapped = (summary?.sources ?? []).filter((s) => s.format !== 'imgt-highvquest').reduce((n, s) => n + Object.keys(s.geneNames ?? {}).length, 0);
  sentences.push(`Gene calls were read in IMGT nomenclature ${cite('imgtNames')}, ambiguous calls kept as sets${mapped ? `; ${plural(mapped, 'Adaptive Biotechnologies gene name')} ${mapped === 1 ? 'was' : 'were'} mapped to IMGT's by ClonoLoom's rules and the HGNC list of human TR and IG genes ${cite('hgnc')}, each mapping recorded` : ''}.`);

  // Validation.
  const f = dataset.findings ?? { fatal: 0, warnings: 0, notes: 0 };
  const warnings = (summary?.report?.findings ?? []).filter((x) => x.severity === 'warning' || x.severity === 'fatal')
    .map((x) => `${x.title.charAt(0).toLowerCase()}${x.title.slice(1)} (${plural(x.count, 'row')})`);
  sentences.push(`ClonoLoom's validator found ${plural(f.fatal, 'fatal problem')}, ${plural(f.warnings, 'warning')} and ${plural(f.notes, 'note')}${warnings.length ? `: ${joinList(warnings)}` : ''}.`);

  // The filter its analyses use.
  if (filter) {
    const kept = filterResult ? `, which kept ${count(filterResult.kept)} of ${count(filterResult.rows)} ${dataset.singleCell ? 'contigs' : 'rearrangements'}${filterResult.rows ? ` (${((100 * filterResult.kept) / filterResult.rows).toFixed(1)}%)` : ''}` : '';
    sentences.push(`${dataset.singleCell ? 'Contigs' : 'Rearrangements'} were filtered with "${filter.name}" (version ${filter.version}: ${describeRules(filter.rules)}; made because "${filter.reason.replace(/[.\s]+$/, '')}")${kept}.`);
  }

  // Clonotypes.
  if (definition && clonotypes) {
    sentences.push(`Clonotypes were defined as ${clonotypes.describe} ("${definition.name}", ClonoLoom definition ${clonotypes.id}).`);
    if (/junction/.test(clonotypes.describe)) sentences.push(`A junction is the IMGT junction, from the conserved 2nd-CYS 104 to J-PHE or J-TRP 118 ${cite('imgtJunction')}.`);
    if (clonotypes.count !== undefined) sentences.push(`${clonotypes.cells ? `The ${plural(clonotypes.cells, 'cell')} formed` : 'They formed'} ${plural(clonotypes.count, 'clonotype')}.`);
  }

  if (definition && clonotypes && ['human', 'mouse'].includes(dataset.invariantSpecies)) {
    sentences.push(`Retained chains were evaluated for ${dataset.invariantSpecies} MAIT and iNKT sequence rules ${cite('conga')}${dataset.invariantSpecies === 'human' ? ` and the GEM alpha-chain motif ${cite('gem')}` : ''} (rule version 1). Junction lengths include the conserved cysteine and terminal phenylalanine or tryptophan. Missing or ambiguous required calls and definitions lacking amino-acid junctions or gene resolution were not evaluable. Human iNKT required a beta chain with TRBV25 in the same clonotype. Each clonotype contributed once per rule to each sample; shares used all retained abundance, including unevaluable receptors, with evaluable abundance reported separately. These sequence flags do not establish cell phenotype or antigen specificity.`);
  }

  // Diversity, as each result kept says it was measured (the latest of each definition and filter).
  const diversities = results.filter((r) => r.kind === 'diversity');
  for (const r of diversities) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    sentences.push(`Each sample's diversity was measured over its clonotypes' abundances in ${unit} (${under}) by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: the Hill numbers of order 0 to 4 ${cite('hill')}, Shannon's entropy (natural log), Simpson's index, Pielou's evenness, clonality (1 − evenness) and Simpson clonality (√Σp²), Gini's coefficient, the largest clonotypes' shares and clonal space homeostasis; the richness of the repertoire each was drawn from by Chao1 (bias-corrected) ${cite('chao1984')} ${cite('chao2014')} and ACE ${cite('chaoLee')}, its Shannon diversity by Chao, Wang and Jost's estimator ${cite('chaoWangJost')} and its Simpson diversity by n(n−1)/ΣX(X−1) ${cite('chao2014')}; and the sample coverage ${cite('chaoJost')}. Intervals are 95%: Chao1's from its variance, log-transformed; the others' from ${opts.replicates ?? 200} bootstrap replicates of each sample drawn from its estimated repertoire ${cite('chao2014')} (seed ${opts.seed ?? '—'}), the estimate ± 1.96 standard errors.`);
  }

  // Rarefaction and extrapolation, likewise, with the levels the samples were compared at.
  for (const r of results.filter((x) => x.kind === 'rarefaction')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const l = r.levels;
    const levels = l ? ` The samples were compared at a sample coverage of ${coverageWords(l.coverage)}, the lowest among them, and at ${Math.round(l.size).toLocaleString('en-US')} ${unit}, the smallest sample's size, each sample rarefied to them.` : '';
    sentences.push(`Each sample's clonotypes, Shannon diversity and Simpson diversity (the Hill numbers of order 0, 1 and 2 ${cite('hill')}) were rarefied and extrapolated, over its clonotypes' abundances in ${unit} (${under}), by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: at sample sizes up to twice its own ${cite('chao2014')}, and at equal sample coverage ${cite('chaoJost')}, with 95% intervals from ${opts.replicates ?? 200} bootstrap replicates of each sample drawn from its estimated repertoire (seed ${opts.seed ?? '—'}), the estimate ± 1.96 standard errors.${levels}`);
  }

  // Gene usage, junction lengths and composition, likewise.
  for (const r of results.filter((x) => x.kind === 'usage')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const groups = r.groups ? ` Each condition was summarized by the mean of its samples' shares, each sample counted once.` : '';
    sentences.push(`What each sample's clonotypes (${under}) are made of was counted by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: their V, D, J and C genes and V–J pairs, at the gene and the IMGT subgroup level ${cite('imgtNames')}, a call tied between several genes sharing its part equally among them; their junctions' lengths in amino acids; and their junctions' amino acids, overall and at each IMGT position ${cite('imgtJunction')}. Each is a share of the sample's clonotypes, each counted once, and of its abundance in ${unit}, with a 95% interval from ${opts.replicates ?? 200} bootstrap replicates of the sample's clonotypes (seed ${opts.seed ?? '—'}), the estimate ± 1.96 standard errors.${groups}`);
  }

  // Overlap, likewise.
  for (const r of results.filter((x) => x.kind === 'overlap')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const depth = r.depth > 0 ? `, as sampled and with both samples rarefied to ${Math.round(r.depth).toLocaleString('en-US')} ${unit}, the smallest sample's size (the exact expectation)` : '';
    const by = r.publicBy === 'subject' ? 'subjects' : 'samples';
    sentences.push(`The overlap of every pair of samples (clonotypes ${under}) was measured by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: the clonotypes they share, Jaccard's index ${cite('jaccard')}, Sørensen–Dice's ${cite('dice')} and the overlap coefficient${depth}; and over their abundances in ${unit}, Morisita's index ${cite('morisita')} and the Morisita–Horn index ${cite('horn')}. Intervals are 95%: from ${opts.replicates ?? 200} bootstrap replicates of each sample's cells (seed ${opts.seed ?? '—'}), widened for the chance that decides which clonotypes two samples share ${cite('horvitzThompson')}, on the logarithmic scale. Clonotypes found in more than one of the ${by} were counted as public.`);
  }

  // Clonotypes followed over time, likewise.
  for (const r of results.filter((x) => x.kind === 'tracking')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const choice = r.choice ?? { by: 'reference', n: 10, list: [] };
    const how = r.intervals
      ? `each one's frequency at each time point is its count of the ${unit} there, the samples taken at one time pooled, with Wilson's ${Math.round(100 * (r.options?.confidence ?? 0.95))}% score interval ${cite('wilson')}, which accounts for sampling alone; a clonotype not found at a time point is reported as not found, the interval's upper end being what that depth could have missed`
      : `each one's frequency at each time point is its share of the ${unit} there, the samples taken at one time pooled, without an interval (${r.unit === 'reads' ? 'reads count each molecule many times over' : 'the abundances are not whole counts'})`;
    sentences.push(`Clonotypes (${under}) were followed through each subject's samples in time order by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: ${choiceWords(choice)}, ${plural(r.followed ?? 0, 'clonotype')} in ${plural(r.subjects ?? 0, 'subject')}; ${how}.`);
  }

  // Clonotypes tested between two time points, likewise.
  for (const r of results.filter((x) => x.kind === 'expansion')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const c = r.comparison ?? {};
    const used = r.used ?? c.test;
    const f = r.fit;
    const on = f?.on === 'stable' ? `the clonotypes of each subject's samples at ${c.stable?.from} and ${c.stable?.to}, two time points taken to be stable` : `the clonotypes of ${plural(f?.pairs ?? 0, 'pair')} of replicate samples, taken at one time`;
    const test = used === 'betabinomial' ? `the beta-binomial test of its count at the first time point given its total in the two, at the first's share of their ${unit}, its scale v(n) = 2 + (b ln n + a)² of Rytlewski et al. ${cite('rytlewski')} (the 2, which keeps the beta unimodal, as in their reference code) fitted by maximum likelihood on ${on} (${plural(f?.clonotypes ?? 0, 'clonotype')} with at least ${c.minTotal ?? 5} in the two; a = ${Number(f?.a).toPrecision(4)}, b = ${Number(f?.b).toPrecision(4)})` : used === 'fisher' ? `Fisher's exact test of its count against the others' at the two time points ${cite('dewitt')}` : `the binomial test of its count at the first time point given its total in the two, at the first's share of their ${unit} ${cite('rytlewski')}, which approximates Fisher's exact test, as DeWitt et al. used it ${cite('dewitt')}, when the clonotype is a small part of the samples`;
    const correction = c.correction === 'storey' ? `Storey's q-values, π₀ estimated at λ = 0.5 ${cite('storey')}` : `Benjamini and Hochberg's procedure ${cite('bh')}`;
    const side = c.alternative === 'increase' ? 'one-sided, for an increase' : c.alternative === 'decrease' ? 'one-sided, for a decrease' : 'two-sided, summing the outcomes no more likely than the one seen';
    const caution = used === 'betabinomial'
      ? ` Between the samples it was fitted on, a two-sided test at 0.05 called ${(100 * (f?.calledBySampling ?? 0)).toFixed(1)}% of the clonotypes changed by sampling alone, and ${(100 * (f?.calledByModel ?? 0)).toFixed(1)}% with the fitted variance; the variance was taken to be the same at every time point and in every subject.`
      : ` The test accounts for sampling alone, not for the variation between samples taken at one time${c.test === 'auto' ? ' (the study had no samples to fit that variance on)' : ''}.`;
    sentences.push(`Each clonotype (${under}) was tested between ${c.from} and ${c.to} in each subject by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}, the samples at one time pooled: ${test}, ${side}. Clonotypes with fewer than ${c.minTotal ?? 5} ${unit} in the two together were not tested ${cite('rytlewski')}. The false discovery rate was controlled within each subject by ${correction}, and clonotypes with an adjusted p-value of at most ${c.alpha ?? 0.01} called increased or decreased: ${plural(r.increased ?? 0, 'clonotype')} increased and ${plural(r.decreased ?? 0, 'clonotype')} decreased of ${plural(r.tested ?? 0, 'clonotype')} tested in ${plural(r.subjects ?? 0, 'subject')}.${caution}`);
  }

  // Clonotypes classed across time points, likewise.
  for (const r of results.filter((x) => x.kind === 'classes')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const c = r.classing ?? {};
    const test = r.used === 'betabinomial' ? `the beta-binomial test ${cite('rytlewski')}, its variance fitted on ${r.fit?.on === 'stable' ? 'two time points taken to be stable' : 'the replicate samples'} (a = ${Number(r.fit?.a).toPrecision(4)}, b = ${Number(r.fit?.b).toPrecision(4)})` : r.used === 'fisher' ? `Fisher's exact test ${cite('dewitt')}` : `the binomial test ${cite('rytlewski')}`;
    const correction = c.correction === 'storey' ? `Storey's q-values ${cite('storey')}` : `Benjamini and Hochberg's procedure ${cite('bh')}`;
    const n = r.counts ?? {};
    sentences.push(`Each clonotype (${under}) was tested between ${c.base} and every later time point of each subject by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}, by ${test}, two-sided, over its ${unit}, the false discovery rate controlled within each comparison by ${correction} at ${c.alpha ?? 0.01}, and classed by its calls: emerging where called increased at the last time point, contracting where called decreased at it, transient where called changed at a time point between but not at the last, and persistent where never called and found at every time point. In ${plural(r.subjects ?? 0, 'subject')}, ${plural(n.emerging ?? 0, 'clonotype')} were emerging, ${count(n.transient ?? 0)} transient, ${count(n.contracting ?? 0)} contracting and ${count(n.persistent ?? 0)} persistent.`);
  }

  // Attribution, likewise.
  for (const r of results.filter((x) => x.kind === 'attribution')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const interval = r.intervals ? `, with Wilson's ${Math.round(100 * (r.options?.confidence ?? 0.95))}% score interval for sampling ${cite('wilson')}` : '';
    const derived = r.derived ? ` For the ${plural(r.derived, 'subject')} whose reference shared few clonotypes with the samples before the reference event (at most 25% of its ${unit}), the share derived from the reference was estimated as the share, of a sample's ${unit} in clonotypes found in the reference or in the samples before but not in both, in the reference's${interval}.` : '';
    sentences.push(`Each sample's share of ${unit} in the clonotypes (${under}) its subject's reference sample (${(r.references ?? []).join(', ') || 'reference'}) holds was measured by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}${interval}; it counts the subject's own cells where the reference holds their clonotypes, and not the clonotypes the reference sample missed.${derived}`);
  }

  // TCRdist between clonotypes.
  for (const r of results.filter((x) => x.kind === 'tcrdist')) {
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const loops = r.cdr3Only
      ? 'their CDR3s alone (the IMGT junction less its first three and last two residues, weighted 3, each residue of difference in length costing 12)'
      : `their CDR3s (the IMGT junction less its first three and last two residues, weighted 3, each residue of difference in length costing 12) and their V genes' CDR1, CDR2 and CDR2.5 (each weighted 1), the loops of each ${r.organism} V allele taken from tcrdist3's database of genes (${r.loops?.file ?? 'combo_xcr_2024-03-05.tsv'}, SHA-256 ${r.loops?.sha256 ?? ''}), made from IMGT's germline genes ${cite('imgtNames')}`;
    sentences.push(`Each T-cell clonotype (${under}) was compared with every other of the same chains by TCRdist ${cite('dash')}, as tcrdist3 computes it ${cite('tcrdist3')}, by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: two residues differing by min(4, 4 − BLOSUM62), a pair of chains by the sum of its chains', each chain by ${loops}. Of ${plural(r.compared ?? 0, 'clonotype')} compared, ${count(r.withNeighbors ?? 0)} had another within ${r.radius} TCRdist units.`);
  }

  // Generation probabilities.
  for (const r of results.filter((x) => x.kind === 'pgen')) {
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const loci = Object.entries(r.loci ?? {}).map(([locus, model]) => `${model} for ${locus}`).join(', ');
    const models = r.models === 'clonoloom'
      ? `ClonoLoom's own model of human TRB recombination (its simulator's: GRCh38's gene segments, version ${String(r.set?.checksum ?? '').split(' ').at(-1) || '1'})`
      : `OLGA's default models ${cite('sethna')}${loci ? ` (${loci})` : ''}, inferred by IGoR ${cite('marcou')} (checksum ${r.set?.checksum ?? ''})`;
    sentences.push(`Each clonotype's generation probability (${under}), the probability that V(D)J recombination alone makes its CDR3s${r.genes ? ' with their V and J genes' : ''}, summed over every gene choice, deletion and insertion that makes them, was computed by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)} as OLGA computes it ${cite('sethna')}, under ${models}, ${r.organism ?? 'human'}; a pair's as the product of its chains'. Of ${plural(r.clonotypes ?? 0, 'clonotype')}, ${count(r.computed ?? 0)} had one.`);
  }

  // Neighborhoods against a background.
  for (const r of results.filter((x) => x.kind === 'neighborhoods')) {
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const background = r.background?.kind === 'shuffled'
      ? `the same receptors with their CDR3s moved among their V and J genes (${count(r.background?.receptors ?? 0)} receptors)`
      : `${count(r.background?.receptors ?? 0)} receptors drawn from ${r.background?.name ?? 'a recombination model'} (checksum ${r.background?.checksum ?? ''}) with the dataset's V–J usage, each weighted by the model's probability of its V–J pair over the dataset's share of it`;
    const loops = r.cdr3Only
      ? 'their CDR3s alone'
      : `their CDR3s and their V genes' CDR1, CDR2 and CDR2.5, the loops of each ${r.organism} V allele taken from tcrdist3's database of genes (${r.loops?.file ?? ''}, SHA-256 ${r.loops?.sha256 ?? ''})`;
    sentences.push(`Each ${r.locus} receptor (${under}) was taken as the centre of a neighborhood by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)} as Mayer-Blackwell et al. define one ${cite('tcrdist3')}: its radius the largest of 0, 2, …, ${r.maxRadius ?? 48} TCRdist units ${cite('dash')} at which the weighted background within it was at most a share ${r.control ?? 1e-6} of the background's receptors, against a background of ${background}, comparing ${loops}. Of ${plural(r.receptors ?? 0, 'receptor')}, ${count(r.centers ?? 0)} had a neighborhood of ${r.minNeighbors ?? 2} receptors or more, each reported with the rates (n + 1)/(N + 2) in the dataset and the background, their ratio, an odds ratio and a χ² test.`);
  }

  for (const r of results.filter((x) => x.kind === 'specificity')) {
    const o = r.specificity;
    const distance = o.metric === 'exact' ? 'identical CDR3 amino-acid sequences' : `TCRdist ≤ ${o.radius} ${cite('dash')} with searched gaps ${cite('tcrdist3')}, ${o.cdr3Only ? 'CDR3s alone' : `including V-gene loops (SHA-256 ${r.loops?.sha256 ?? ''})`}`;
    sentences.push(`Retained chains (${r.describe || r.definition}) were matched at the same locus to ${r.database.reference} (${r.database.source}; SHA-256 ${r.database.sha256}; ${r.database.license}) ${cite(`specificityDatabase:${r.database.reference}`, r.database.citation)}, using ${distance}${o.genes ? ' and identical nonempty V/J call sets at the clonotype definition’s resolution' : ''}, receptor species ${o.species} and minimum database score ${o.minScore}. The ${r.matches} chain–record matches retained their evidence; each clonotype contributed once to each epitope/antigen group’s abundance, divided by all retained abundance per sample. These overlapping shares describe database evidence, not established specificity of paired receptors.`);
  }

  for (const r of results.filter((x) => x.kind === 'isotypes')) {
    const o = r.isotypes ?? {};
    sentences.push(`Observed isotype usage (${r.id}; ${r.assigned} assigned observations; class resolved for ${r.resolvedClass}, constant gene resolved for ${r.resolvedGene}; observation unit ${r.unit}, abundance unit ${r.abundanceUnit || 'unspecified'}) used saved families ${o.families}, SHA-256 ${o.familySha256}. Constant calls follow AIRR c_call ${cite('airr')} and IMGT nomenclature ${cite('imgtNomenclature', 'Lefranc MP. Immunoglobulin and T cell receptor genes: IMGT and the birth and rise of immunoinformatics. Front Immunol. 2014;5:22. doi:10.3389/fimmu.2014.00022')}. ${r.describe} Counting, ambiguity and missingness rules are ClonoLoom policies. Biological context follows ${cite('bcellSwitching', 'Horns F, Vollmers C, Croote D, et al. Lineage tracing of human B cells reveals the in vivo landscape of human antibody class switching. eLife. 2016;5:e16578. doi:10.7554/eLife.16578')}; no ancestral-trait or switching statistic from ${cite('bcellTraits', 'Hoehn KB, Pybus OG, Kleinstein SH. Phylogenetic analysis of migration, differentiation, and class switching in B cells. PLoS Comput Biol. 2022;18:e1009885. doi:10.1371/journal.pcbi.1009885')} was calculated.`);
  }
  for (const r of results.filter((x) => x.kind === 'lineages')) {
    const o = r.lineages ?? {};
    sentences.push(`B-cell lineage trees (${r.id}; ${r.built} family/locus trees) used saved germlines ${o.germlines}, SHA-256 ${o.germlineSha256}. Equal-cost nucleotide parsimony follows ${cite('fitch', 'Fitch WM. Toward Defining the Course of Evolution: Minimum Change for a Specific Tree Topology. Syst Zool. 1971;20:406–416. doi:10.1093/sysbio/20.4.406')}; conditional ancestral costs follow ${cite('sankoff', 'Sankoff D. Minimal Mutation Trees of Sequences. SIAM J Appl Math. 1975;28:35–42. doi:10.1137/0128004')}. ${r.describe} Panel selection, search bounds and deterministic tie handling are ClonoLoom policies. Observed annotations are pinned in the result. No class-switching or migration statistic from ${cite('bcellTraits', 'Hoehn KB, Pybus OG, Kleinstein SH. Phylogenetic analysis of migration, differentiation, and class switching in B cells. PLoS Comput Biol. 2022;18:e1009885. doi:10.1371/journal.pcbi.1009885')} was calculated.`);
    const p = r.germlineProvenance ?? {};
    sentences.push(`The germline parent used families ${p.families}, SHA-256 ${p.familySha256}.`);
    for (const f of p.files ?? []) sentences.push(`Germline reference ${f.reference}, ${f.file}, SHA-256 ${f.sha256}, ${f.license}: ${cite(`germline:${f.reference}`, f.citation)}; ${f.source}.`);
  }
  for (const r of results.filter((x) => x.kind === 'shm')) {
    const o = r.shm ?? {};
    sentences.push(`Somatic hypermutation (${r.id}; ${r.evaluated} of ${r.rowCount} source rows with evaluable bases) compared sequences with saved D-masked germlines ${o.germlines}, SHA-256 ${o.germlineSha256}, following the comparison framework of ${cite('changeo', 'Gupta NT, Vander Heiden JA, Uduman M, et al. Change-O. Bioinformatics. 2015;31:3356–3358. doi:10.1093/bioinformatics/btv359')}. ${r.describe} These conservative evaluability and attribution rules are ClonoLoom policies. AIRR query coordinates ${cite('airr')} and NCBI genetic code table 1 ${cite('geneticCode', 'NCBI. The Genetic Codes, table 1. https://www.ncbi.nlm.nih.gov/Taxonomy/Utils/wprintgc.cgi#SG1')} determine region and codon comparisons. Unclassified substitutions remain in nucleotide frequencies. No selection statistic was calculated.`);
    const p = r.germlineProvenance ?? {};
    sentences.push(`The germline parent used families ${p.families}, SHA-256 ${p.familySha256}.`);
    for (const f of p.files ?? []) sentences.push(`Germline reference ${f.reference}, ${f.file}, SHA-256 ${f.sha256}, ${f.license}: ${cite(`germline:${f.reference}`, f.citation)}; ${f.source}.`);
  }
  for (const r of results.filter((x) => x.kind === 'germlines')) {
    const o = r.germlines ?? {};
    sentences.push(`D-masked germline reconstruction (${r.id}; ${r.reconstructed} of ${r.rowCount} assigned source rows) used saved families ${o.families}, SHA-256 ${o.familySha256}. ${r.describe} The masking framework follows ${cite('changeo', 'Gupta NT, Vander Heiden JA, Uduman M, et al. Change-O. Bioinformatics. 2015;31:3356–3358. doi:10.1093/bioinformatics/btv359')}; family allele intersection is a conservative ClonoLoom policy. Query coordinates follow AIRR Rearrangement schema 2.0 ${cite('airr')} on the oriented sequence; ungapped reference coordinates are retained separately. The supplied germline alignment was not used as the reconstructed reference. No mutation rate or lineage ancestor was inferred.`);
    for (const f of o.files ?? []) sentences.push(`Germline reference ${f.reference}, ${f.file}, SHA-256 ${f.sha256}, ${f.license}: ${cite(`germline:${f.reference}`, f.citation)}; ${f.source}.`);
  }
  for (const r of results.filter((x) => x.kind === 'families')) {
    sentences.push(`B-cell family analysis (${r.id}; ${r.eligible} eligible observations, ${r.representativeCount} distinct receptors, ${r.familyCount} families) used length-normalized nucleotide Hamming distances and V/J gene and junction-length partitions ${cite('changeo', 'Gupta NT, Vander Heiden JA, Uduman M, et al. Change-O: a toolkit for analyzing large-scale B cell immunoglobulin repertoire sequencing data. Bioinformatics. 2015;31:3356–3358. doi:10.1093/bioinformatics/btv359')} and ${r.families?.threshold == null ? 'nearest-distance diagnostics described by' : 'single linkage following'} ${cite('bcellFamilies', 'Gupta NT, Adams KD, Briggs AW, et al. Hierarchical Clustering Can Identify B Cell Clones with High Confidence in Ig Repertoire Sequencing Data. J Immunol. 2017;198:2489–2499. doi:10.4049/jimmunol.1601850')}. ${r.describe} The recorded QC filter was ${r.filterDescription || 'recorded in the result'}. Where present, cell IDs were scoped by sample; duplicate identical chains were collapsed, and ambiguous or missing evidence was excluded explicitly. Paired maximum distance is a conservative ClonoLoom policy, used only in paired mode. Source identities, exclusions, subject annotations and sample abundance are retained in the result; abundance does not weight the distance distribution.`);
  }

  for (const r of results.filter((x) => x.kind === 'networks')) {
    const o = r.networks;
    const distance = o.metric === 'tcrdist'
      ? `TCRdist ${cite('dash')} with the gap searched ${cite('tcrdist3')}, ${o.cdr3Only ? 'CDR3s alone' : `including V-gene loops (${r.organism}, SHA-256 ${r.loops?.sha256 ?? ''})`}`
      : o.metric === 'levenshtein' ? `Levenshtein distance ${cite('levenshtein', 'Levenshtein VI. Binary codes capable of correcting deletions, insertions, and reversals. Soviet Physics Doklady. 1966;10:707–710.')}, on whole junctions` : `${o.metric} on whole junctions`;
    sentences.push(`A clonotype network (${r.describe || r.definition}) joined ${r.nodes} clonotypes by ${distance} at cutoff ${o.cutoff}, requiring ${o.arms === 'all' ? 'a bijection of all retained chains at each locus (missing matching missing only)' : 'any shared chain at the same locus'}${o.sameV ? ', with matching V calls' : ''}${o.sameJ ? ', with matching J calls' : ''}. The ${r.edges} edges formed ${r.components} connected components. Components express sequence similarity, not proven antigen specificity. A deterministic force drawing ${cite('forceDrawing', 'Fruchterman TMJ, Reingold EM. Graph drawing by force-directed placement. Software: Practice and Experience. 1991;21:1129–1164. doi:10.1002/spe.4380211102')} used a fixed grid start and 60 iterations, with a drawing limit independent of the complete graph.`);
  }

  // These descriptions and citations are computed in Go and kept with each immutable result.
  // Do not reconstruct scientific parameters from current controls or a plot selection.
  for (const r of results.filter((r) => ['pseudobulk-de', 'pseudobulk', 'batch-qc', 'contamination', 'convergence', 'motifs', 'expression-map', 'phenotypes', 'modularity', 'conga', 'anndata-export', 'cohort-features', 'cohort-compare', 'definition-robustness', 'cohort-counts'].includes(r.kind))) {
    sentences.push(`Saved ${r.method || r.kind} result ${r.id}: ${r.describe || 'See the saved result for its effective parameters.'}${r.citation ? ` ${cite(`cell-method:${r.citation}`, r.citation)}` : ''}`);
  }
  return { text: sentences.join(' '), references: refs.map((r, i) => `[${i + 1}] ${r.text}`) };
}

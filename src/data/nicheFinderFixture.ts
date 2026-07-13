import type { CreatorNicheProfile, NicheCandidate, NicheReport } from '../lib/api/client';

const profile: CreatorNicheProfile = {
  professional_skills: 'software development, AI automation, workflow systems',
  hobbies: 'productivity systems, small business operations, creator tooling',
  lived_experiences: 'building automation tools for teams and explaining technical systems to non-technical operators',
  teaching_subjects: 'AI workflows, no-code automation, software systems, prompt operations',
  three_years_ago_advice: 'how to automate repetitive business tasks without creating fragile systems',
  target_audience: 'UK small businesses and solo operators',
  target_country: 'GB',
  target_language: 'en',
  creator_presence: 'faceless channel',
  content_formats: ['long-form'],
  optional_broad_topic: 'AI automation for service businesses',
  weekly_production_capacity: '2 videos per week',
};

const dimensionExplanations = {
  creator_fit: 'Strong fit because the creator can demonstrate credible workflows from real software and automation experience, with enough operational context to explain tradeoffs instead of only listing tools.',
  audience_demand: 'High demand is supported by recurring search behavior around AI tools, workflow automation, and small-business efficiency. The long-tail queries are specific enough to support practical tutorials.',
  competition_opportunity: 'The market has broad AI content but a visible gap for calm, operator-focused implementation guides aimed at UK service businesses with limited technical staff.',
  sustainability: 'A complete runway is available because each core workflow can generate setup, troubleshooting, comparison, template, and case-study formats across several business functions.',
  differentiation: 'Differentiation comes from combining practical systems thinking, UK business context, and faceless tutorial packaging rather than competing with broad AI news channels.',
};

function dimensions(scores: [number, number, number, number, number]) {
  return {
    creator_fit: { score: scores[0], label: 'Strong', rating_band: 'very_high', explanation: dimensionExplanations.creator_fit },
    audience_demand: { score: scores[1], label: 'High', rating_band: 'high', explanation: dimensionExplanations.audience_demand },
    competition_opportunity: { score: scores[2], label: 'Promising', rating_band: 'high', explanation: dimensionExplanations.competition_opportunity },
    sustainability: { score: scores[3], label: 'Durable', rating_band: 'very_high', explanation: dimensionExplanations.sustainability },
    differentiation: { score: scores[4], label: 'Clear', rating_band: 'high', explanation: dimensionExplanations.differentiation },
  };
}

const pillars = [
  { name: 'Client operations', description: 'Lead capture, onboarding, follow-up and status workflows.', percentage: 30, topic_count: 15 },
  { name: 'Admin automation', description: 'Internal documents, inboxes, scheduling and reporting.', percentage: 24, topic_count: 12 },
  { name: 'AI tool reviews', description: 'Practical comparisons for business owners.', percentage: 18, topic_count: 9 },
  { name: 'Templates and SOPs', description: 'Reusable systems viewers can copy.', percentage: 16, topic_count: 8 },
  { name: 'Risk and QA', description: 'Accuracy, handoff and review safeguards.', percentage: 12, topic_count: 6 },
];

const titleSeeds = [
  ['Automate every new client enquiry without hiring an assistant', 'Client operations', 'implementation guide'],
  ['Build a simple AI inbox triage system for a two-person service business', 'Admin automation', 'tutorial'],
  ['The safest way to use ChatGPT for customer follow-up emails', 'Risk and QA', 'explainer'],
  ['Turn a messy spreadsheet into a weekly operations dashboard', 'Admin automation', 'walkthrough'],
  ['Five AI automations a UK accountant can set up this week', 'Client operations', 'use case'],
  ['Compare Zapier, Make, and simple scripts for small-business automation', 'AI tool reviews', 'comparison'],
  ['Create a client onboarding checklist that updates itself', 'Templates and SOPs', 'template'],
  ['How to stop AI automations from sending the wrong message', 'Risk and QA', 'quality guide'],
  ['Build a booking reminder workflow that reduces no-shows', 'Client operations', 'tutorial'],
  ['The no-code CRM workflow I would build for a local agency', 'Templates and SOPs', 'case study'],
];

const recommendedTitles = Array.from({ length: 50 }, (_, index) => {
  const seed = titleSeeds[index % titleSeeds.length];
  const suffix = index < 10 ? '' : `, part ${Math.floor(index / 10) + 1}`;
  return {
    title: `${seed[0]}${suffix}`,
    pillar: seed[1],
    intent: seed[2],
    difficulty: index % 4 === 0 ? 'medium' : 'low',
    source: index % 3 === 0 ? 'validated demand pattern' : 'runway expansion',
    evidence_status: index < 18 ? 'evidence-backed' : 'related opportunity',
  };
});

function candidate(id: string, name: string, score: number, scoreSet: [number, number, number, number, number], audience: string): NicheCandidate {
  return {
    id,
    name,
    concise_positioning: 'A practical education channel for owners who want reliable AI automation without enterprise complexity or generic tool hype.',
    category: 'Business automation',
    subcategory: 'AI workflow systems',
    target_audience: audience,
    audience_problems: [
      'Owners lose hours to lead follow-up, admin, reporting and repetitive status communication.',
      'Most AI tutorials assume either enterprise teams or technical builders, leaving operators without a reliable implementation path.',
      'Viewers need plain-English safeguards before trusting automation with customer-facing work.',
    ],
    creator_advantages: [
      'Can explain software tradeoffs clearly and show complete systems rather than isolated prompts.',
      'Can create faceless screen-recorded demos at a sustainable weekly production pace.',
      'Has direct credibility in automation and product workflow design.',
    ],
    unique_angle: 'Calm, evidence-led AI automation tutorials for UK service businesses, focused on durable workflows and operational safeguards.',
    overall_score: score,
    confidence: 'medium',
    dimensions: dimensions(scoreSet),
    content_pillars: pillars,
    recommended_titles: recommendedTitles,
    opportunity_gaps: [
      'Few channels package AI automation around UK service-business workflows and regulatory language.',
      'Most competitors publish tool news; fewer show end-to-end implementation with checks and handoff points.',
      'Search demand is spread across practical workflow jobs, allowing repeatable long-tail content.',
    ],
    evidence_summary: 'Fixture evidence models a live-validated result with enough sample size to stress layout, wrapping, timestamps and long explanatory strings without calling OpenAI or YouTube.',
    market_evidence: {
      status: 'live_validated',
      source_types: ['youtube_search', 'trend_context'],
      sample_size: 42,
      recent_activity: 'Validated videos published within the last 21 days',
      median_views: 18400,
      engagement: 4.8,
      collected_at: '2026-07-12T09:30:00Z',
      limitations: ['Fixture data is deterministic for visual QA and does not represent a fresh production research run.'],
    },
    search_queries_used: [
      'AI automation for small business UK',
      'automate client onboarding with AI',
      'Zapier Make ChatGPT workflow tutorial',
      'AI workflow for service business',
      'small business admin automation',
    ],
    runway: {
      viable_topic_count: 50,
      estimated_weeks: 25,
      weekly_capacity: 2,
      estimated_content_runway: 'A complete 25-week content runway at your stated capacity.',
      heading: 'Content runway',
      limitation: 'Runway assumes two long-form videos per week and does not change the backend calculation.',
    },
    level_1: 'Business',
    level_2: 'Automation',
    level_3: 'AI workflows',
    niche_name: name,
    core_phrase: 'AI automation for UK small businesses',
    target_viewer: audience,
    viewer_problem: 'Time-poor operators need reliable workflows and plain-English safeguards.',
    creator_advantage: 'Software and AI automation credibility with practical teaching range.',
    recommended_content_format: 'Long-form faceless tutorials with templates',
    validation: {
      search_phrases: ['AI automation for small business', 'client onboarding automation', 'ChatGPT workflow tutorial'],
      recent_publication_volume: 38,
      sampled_video_count: 42,
      total_sampled_views: 928000,
      median_sampled_views: 18400,
      engagement_rate: 4.8,
      newest_activity: '2026-07-10T12:00:00Z',
      rising_topic_overlap: true,
      market_evidence_summary: 'Recent tutorial demand concentrates around practical business workflows and tool comparisons.',
      competition_level: 'moderate',
      validation_budget_used: 1,
      evidence_confidence: 'high',
    },
    outliers: [
      {
        title: 'I Automated My Client Onboarding System With ChatGPT, Make, and One Spreadsheet',
        thumbnail_url: 'https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg',
        canonical_url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
        channel_name: 'Workflow Lab',
        publication_age: '12 days old',
        public_views: 128000,
        outlier_reason: 'High views for a practical implementation topic with a narrow operator audience.',
        outlier_strength: 86,
      },
      {
        title: 'The AI Admin System I Use to Save 8 Hours Every Week',
        thumbnail_url: 'https://i.ytimg.com/vi/9bZkp7q19f0/hqdefault.jpg',
        canonical_url: 'https://www.youtube.com/watch?v=9bZkp7q19f0',
        channel_name: 'Operator OS',
        publication_age: '3 weeks old',
        public_views: 74000,
        outlier_reason: 'Clear pain-point framing, useful promise, and repeatable business workflow angle.',
        outlier_strength: 78,
      },
    ],
    supply_gaps: [
      { statement: 'Localized UK examples and terminology are underrepresented.', confidence: 'medium' },
      { statement: 'Most videos stop before QA, handoff and failure-mode handling.', confidence: 'high' },
    ],
    video_topics: recommendedTitles,
    topic_pillars: pillars,
    first_10_titles: recommendedTitles.slice(0, 10).map(topic => topic.title),
    sustainability: {
      viable_topic_count: 50,
      content_pillar_count: 5,
      topic_repetition_risk: 'low',
      estimated_content_runway: '25 production weeks',
      score: scoreSet[3],
      deduped_removed: 6,
    },
    scores: {
      personal_fit: { score: scoreSet[0], label: 'Strong', rating_band: 'very_high', explanation: dimensionExplanations.creator_fit },
      demand: { score: scoreSet[1], label: 'High', rating_band: 'high', explanation: dimensionExplanations.audience_demand },
      opportunity_gap: { score: scoreSet[2], label: 'Promising', rating_band: 'high', explanation: dimensionExplanations.competition_opportunity },
      sustainability: { score: scoreSet[3], label: 'Durable', rating_band: 'very_high', explanation: dimensionExplanations.sustainability },
      overall: { score, label: 'High', rating_band: 'high', explanation: 'Weighted composite score from fit, demand, opportunity and runway.' },
      confidence: { score: 82, label: 'High', rating_band: 'high', explanation: 'Fixture confidence based on complete evidence fields.' },
    },
    risks: [
      'Tool interfaces change quickly, so evergreen videos need periodic review.',
      'Broad AI automation claims can look generic unless every topic is tied to a concrete workflow.',
      'Customer-facing automation examples need careful QA language and human approval checkpoints.',
    ],
    recommended_first_action: 'Publish one implementation video showing a complete client-enquiry workflow, then validate audience retention before expanding into templates.',
  };
}

const primary = candidate('fixture-primary', 'AI Automation Systems for UK Service Businesses', 86, [92, 84, 78, 90, 82], 'UK service-business owners, solo operators and small teams');

export const nicheFinderFixtureReport: NicheReport = {
  id: 'fixture-niche-report',
  schema_version: 'niche_report_v3_relevance_confidence_alternatives',
  status: 'ok',
  message: 'Fixture response for local visual QA.',
  generated_at: '2026-07-12T09:30:00Z',
  evidence_freshness: 'live',
  analysis_mode: 'live_validated',
  provider_status: [
    { id: 'youtube_data_api', name: 'YouTube Data API', platform: 'youtube', status: 'active', message: 'Fixture provider active' },
    { id: 'google_trends_rss', name: 'Google Trends RSS', platform: 'google', status: 'active', message: 'Fixture provider active' },
  ],
  creator_profile_summary: 'Software and AI automation creator targeting UK small businesses with practical, faceless long-form workflow education.',
  primary_recommendation: primary,
  candidates: [
    primary,
    candidate('fixture-alt-1', 'No-Code AI Operations for Local Agencies', 79, [86, 78, 74, 82, 76], 'local agencies and consultancies'),
    candidate('fixture-alt-2', 'AI Admin Systems for Trades and Field Services', 76, [82, 75, 80, 74, 70], 'trades, field-service owners and office managers'),
    candidate('fixture-alt-3', 'Prompt QA Workflows for Small Teams', 73, [88, 70, 66, 72, 79], 'small teams adopting AI for internal operations'),
  ],
  alternative_candidates: [
    candidate('fixture-alt-1', 'No-Code AI Operations for Local Agencies', 79, [86, 78, 74, 82, 76], 'local agencies and consultancies'),
    candidate('fixture-alt-2', 'AI Admin Systems for Trades and Field Services', 76, [82, 75, 80, 74, 70], 'trades, field-service owners and office managers'),
    candidate('fixture-alt-3', 'Prompt QA Workflows for Small Teams', 73, [88, 70, 66, 72, 79], 'small teams adopting AI for internal operations'),
  ],
  methodology: [
    'Profile inputs are mapped to creator credibility, audience, format and production capacity.',
    'Candidate scores combine creator fit, audience demand, competition opportunity, sustainability and differentiation.',
    'Runway depth is represented as usable topic count divided by stated weekly production capacity.',
    'Evidence fields are kept separate from model-derived strategic scores.',
  ],
  profile,
  cache: {
    hit: false,
    cache_hit: false,
    schema_version: 'niche_report_v3_relevance_confidence_alternatives',
    key: 'fixture',
    stored_at: '2026-07-12T09:30:00Z',
    ttl: '6h',
    evidence_fetched_at: '2026-07-12T09:30:00Z',
    evidence_age: '0h',
    freshness: 'fresh',
  },
  limitations: ['This is a deterministic local fixture for Phase 1 visual approval and does not consume production OpenAI or YouTube quota.'],
  created_at: '2026-07-12T09:30:00Z',
};

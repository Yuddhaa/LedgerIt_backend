-- This extension is needed for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create the foundational users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT,
    email TEXT UNIQUE NOT NULL,
    phone_number TEXT UNIQUE,
    google_id TEXT UNIQUE NOT NULL,
    picture TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- refresh_tokens table
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    device_info TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);

-- business tables, types, and functions
CREATE TYPE business_role AS ENUM ('creator', 'admin', 'employee');

CREATE TABLE businesses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT unique_business_name_per_owner UNIQUE (name, owner_id)
);
CREATE INDEX idx_businesses_owner_id ON businesses(owner_id);

CREATE TABLE business_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    role business_role NOT NULL,
    current_balance DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A user can only be in one role per business
    CONSTRAINT unique_user_business UNIQUE (user_id, business_id)
);
CREATE INDEX idx_business_members_user_id ON business_members(user_id);
CREATE INDEX idx_business_members_business_id ON business_members(business_id);


-- parties table
CREATE TABLE parties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    place TEXT NOT NULL,
    phone_number TEXT,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unique_party_name_per_business UNIQUE (business_id, name)
);

-- Indexes
CREATE INDEX idx_parties_business_id ON parties(business_id);
CREATE INDEX idx_parties_place ON parties(place);
CREATE INDEX idx_parties_business_id_place ON parties(business_id, place);

-- transaction ENUM types
CREATE TYPE transaction_direction AS ENUM ('in', 'out');
CREATE TYPE transaction_mode AS ENUM ('online', 'cash', 'cheque');
CREATE TYPE deposit_status AS ENUM ('pending', 'approved', 'rejected');

-- transaction_categories table
CREATE TABLE transaction_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT unique_business_category_name UNIQUE (business_id, name)
);

CREATE INDEX idx_transaction_categories_business_id ON transaction_categories(business_id);

-- transactions table
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT, -- User who entered it
    amount DECIMAL(10, 2) NOT NULL,
    direction transaction_direction NOT NULL,
    category_id UUID REFERENCES transaction_categories(id) ON DELETE SET NULL,
    party_id UUID NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    mode transaction_mode NOT NULL,
    receipt_no TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_business_id ON transactions(business_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_category_id ON transactions(category_id);
CREATE INDEX idx_transactions_party_id ON transactions(party_id);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);

-- Create the new ENUM type
CREATE TYPE transaction_change_type AS ENUM ('edit', 'delete');
CREATE TYPE edit_request_status AS ENUM ('pending', 'approved', 'rejected');

-- transaction_edit_requests table
CREATE TABLE transaction_edit_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID UNIQUE NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    requested_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status edit_request_status NOT NULL DEFAULT 'pending',
    
    -- New column added here
    type transaction_change_type NOT NULL,

    requested_changes JSONB NOT NULL, -- JSONB is preferred over JSON
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transaction_edit_requests_transaction_id ON transaction_edit_requests(transaction_id);
CREATE INDEX idx_transaction_edit_requests_requested_by_id ON transaction_edit_requests(requested_by_id);
CREATE INDEX idx_transaction_edit_requests_reviewed_by_id ON transaction_edit_requests(reviewed_by_id);
CREATE INDEX idx_transaction_edit_requests_status ON transaction_edit_requests(status);
-- Optional: Index on the new type column if you plan to filter by it often (e.g. "show me all delete requests")
CREATE INDEX idx_transaction_edit_requests_type ON transaction_edit_requests(type);
-- deposits table
CREATE TABLE deposits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    depositer_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount DECIMAL(10, 2) NOT NULL,
    remarks TEXT,
    status deposit_status NOT NULL DEFAULT 'pending',
    reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deposits_business_id ON deposits(business_id);
CREATE INDEX idx_deposits_depositer_id ON deposits(depositer_id);
CREATE INDEX idx_deposits_reviewer_id ON deposits(reviewer_id);
CREATE INDEX idx_deposits_status ON deposits(status);

-- subscription related

CREATE TYPE plans_period AS ENUM ('monthly','yearly','permanent');

CREATE TABLE plans (
  id TEXT PRIMARY KEY,
  razorpay_plan_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,

  amount BIGINT NOT NULL,
  currency TEXT NOT NULL DEFAULT 'INR',

  user_limit INT NOT NULL,

  period plans_period NOT NULL DEFAULT 'month',
  active BOOLEAN DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE marketers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,

  offer_code_upi TEXT UNIQUE NOT NULL,
  razorpay_offer_id_upi TEXT NOT NULL,
  offer_code_card TEXT UNIQUE NOT NULL,
  razorpay_offer_id_card TEXT NOT NULL,
  offer_code_life TEXT UNIQUE NOT NULL,
  razorpay_offer_id_life TEXT NOT NULL,

    discount_percent INT NOT NULL DEFAULT 10,
  commission_percent INT NOT NULL CHECK (commission_percent BETWEEN 0 AND 40),
  commission_balance BIGINT NOT NULL DEFAULT 0, 
  total_commission BIGINT NOT NULL DEFAULT 0,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- used as of now 
-- inactive -- in business table as a default
-- pending -- in the subscripitons table as a default, also 'pending' is set when 'subscription.pending' webhook is arrived.
-- trialing -- to show the business is in trial phase
-- trial_ended -- to show the business trial has ended
-- active -- business has active subscription
-- past_due -- subscription is past_due for payment. 
        -- This will be set after the subscription_end_period+1 hour exceeds or  when 'subscription.halted' webhook arrives.
-- paused -- when 'subscription.paused' webhook arrives.
-- canceled -- when 'subscription.cancelled' webhook arrives(either user manually cancels through in-app, or while upgrading the plan or user cancles through UPI/card)
CREATE TYPE subscriptions_status AS ENUM ( 'inactive', 'pending', 'trialing','trialing_pending', 'active', 
    'past_due', 'paused', 'canceled', 'expired', 'authenticated', 'trial_ended'
);

CREATE TABLE subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL REFERENCES plans(id),

  marketer_id UUID REFERENCES marketers(id) ON DELETE SET NULL,

  razorpay_subscription_id TEXT UNIQUE NOT NULL,
  current_period_start TIMESTAMPTZ,
  current_period_end TIMESTAMPTZ,

  status subscriptions_status NOT NULL, 

  is_offer_applied BOOLEAN NOT NULL DEFAULT false,
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscriptions_business_id ON subscriptions(business_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_rzp_id ON subscriptions(razorpay_subscription_id);

CREATE TYPE subscription_invoices_status AS ENUM (
    'pending', 'paid', 'failed', 'refunded'
);

CREATE TABLE subscription_invoices (
  id UUID PRIMARY key DEFAULT gen_random_uuid(),
  subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE SET NULL,
  business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,

  razorpay_payment_id TEXT NOT NULL UNIQUE,
  amount_paid BIGINT NOT NULL, 
  currency TEXT DEFAULT 'INR',

  status subscription_invoices_status NOT NULL,
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoices_business_id ON subscription_invoices(business_id);

CREATE TYPE marketer_payouts_method AS ENUM ('UPI', 'NEFT', 'CASH');

CREATE TABLE marketer_payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    marketer_id UUID NOT NULL REFERENCES marketers(id) ON DELETE CASCADE,
    
    amount BIGINT NOT NULL,      
    payment_method TEXT NOT NULL, 
    transaction_reference TEXT NOT NULL,   
    
    paid_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payouts_marketer_id ON marketer_payouts(marketer_id);


ALTER TABLE businesses 
ADD COLUMN current_plan_id TEXT REFERENCES plans(id) DEFAULT NULL,
ADD COLUMN subscriptions_status subscriptions_status NOT NULL DEFAULT 'inactive',
ADD COLUMN subscription_end_period TIMESTAMPTZ,
ADD COLUMN is_trial_used BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN current_subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
ADD COLUMN is_offer_used BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN offer_code TEXT;

--- funcitons ---


CREATE OR REPLACE FUNCTION create_business_and_add_owner(
    p_owner_id UUID,
    p_name TEXT
)
RETURNS businesses -- Returns the complete new business row
LANGUAGE plpgsql
AS $$
DECLARE
    new_business businesses;
BEGIN
    -- Create the business
    INSERT INTO businesses (name, owner_id)
    VALUES (p_name, p_owner_id)
    RETURNING * INTO new_business;

    -- Add the owner as a 'creator'
    INSERT INTO business_members (user_id, business_id, role)
    VALUES (p_owner_id, new_business.id, 'creator');

    -- Return the business created in the first step
    RETURN new_business;
END;
$$;

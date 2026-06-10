"use strict";

const { expect } = require("chai");
const { ethers } = require("hardhat");

const HTS_PRECOMPILE = "0x0000000000000000000000000000000000000167";

// Stable bytes32 league ID for tests
const LEAGUE_ID = ethers.encodeBytes32String("test-league-1");
const USDC = (dollars) => ethers.parseUnits(String(dollars), 6);

async function plantMockHTS(provider) {
  // Deploy MockHTS to a throwaway address, then copy its runtime bytecode
  // to the HTS precompile slot so LeagueEscrow's associateToken calls succeed.
  const MockHTS = await ethers.getContractFactory("MockHTS");
  const tmp = await MockHTS.deploy();
  await tmp.waitForDeployment();
  const code = await provider.getCode(await tmp.getAddress());
  await provider.send("hardhat_setCode", [HTS_PRECOMPILE, code]);
}

describe("LeagueEscrow", function () {
  let escrow, usdc;
  let owner, member1, member2, nonMember;

  beforeEach(async function () {
    [owner, member1, member2, nonMember] = await ethers.getSigners();

    await plantMockHTS(ethers.provider);

    const MockUSDC = await ethers.getContractFactory("MockUSDC");
    usdc = await MockUSDC.deploy();
    await usdc.waitForDeployment();

    const LeagueEscrow = await ethers.getContractFactory("LeagueEscrow");
    escrow = await LeagueEscrow.deploy(await usdc.getAddress());
    await escrow.waitForDeployment();

    // Give each test member $1000 USDC
    await usdc.mint(member1.address, USDC(1000));
    await usdc.mint(member2.address, USDC(1000));
  });

  // ─── payEntryFee ─────────────────────────────────────────────────────────────

  describe("payEntryFee", function () {
    it("records payment and updates leagueTotals", async function () {
      await usdc.connect(member1).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(50));

      expect(await escrow.payments(LEAGUE_ID, member1.address)).to.equal(USDC(50));
      expect(await escrow.leagueTotals(LEAGUE_ID)).to.equal(USDC(50));
    });

    it("emits EntryFeePaid", async function () {
      await usdc.connect(member1).approve(await escrow.getAddress(), USDC(50));
      await expect(escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(50)))
        .to.emit(escrow, "EntryFeePaid")
        .withArgs(LEAGUE_ID, member1.address, USDC(50));
    });

    it("accumulates multiple payments from the same member", async function () {
      await usdc.connect(member1).approve(await escrow.getAddress(), USDC(100));
      await escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(60));
      await escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(40));

      expect(await escrow.payments(LEAGUE_ID, member1.address)).to.equal(USDC(100));
    });

    it("reverts when amount is zero", async function () {
      await expect(escrow.connect(member1).payEntryFee(LEAGUE_ID, 0))
        .to.be.revertedWith("amount must be > 0");
    });

    it("reverts when USDC allowance is insufficient", async function () {
      // No approval given
      await expect(escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(50)))
        .to.be.revertedWith("insufficient allowance");
    });
  });

  // ─── claimRefund ─────────────────────────────────────────────────────────────

  describe("claimRefund", function () {
    beforeEach(async function () {
      // member1 pays $50; member2 pays $50
      await usdc.connect(member1).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(50));

      await usdc.connect(member2).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member2).payEntryFee(LEAGUE_ID, USDC(50));
    });

    it("returns USDC to the member", async function () {
      const before = await usdc.balanceOf(member1.address);
      await escrow.connect(member1).claimRefund(LEAGUE_ID);
      expect(await usdc.balanceOf(member1.address)).to.equal(before + USDC(50));
    });

    it("clears the member payment slot to zero", async function () {
      await escrow.connect(member1).claimRefund(LEAGUE_ID);
      expect(await escrow.payments(LEAGUE_ID, member1.address)).to.equal(0);
    });

    it("decrements leagueTotals", async function () {
      await escrow.connect(member1).claimRefund(LEAGUE_ID);
      expect(await escrow.leagueTotals(LEAGUE_ID)).to.equal(USDC(50));
    });

    it("emits RefundClaimed", async function () {
      await expect(escrow.connect(member1).claimRefund(LEAGUE_ID))
        .to.emit(escrow, "RefundClaimed")
        .withArgs(LEAGUE_ID, member1.address, USDC(50));
    });

    it("reverts when the caller has never paid", async function () {
      await expect(escrow.connect(nonMember).claimRefund(LEAGUE_ID))
        .to.be.revertedWith("nothing to refund");
    });

    it("prevents double-refund", async function () {
      await escrow.connect(member1).claimRefund(LEAGUE_ID);
      await expect(escrow.connect(member1).claimRefund(LEAGUE_ID))
        .to.be.revertedWith("nothing to refund");
    });

    it("does not affect other members payments", async function () {
      await escrow.connect(member1).claimRefund(LEAGUE_ID);
      expect(await escrow.payments(LEAGUE_ID, member2.address)).to.equal(USDC(50));
    });

    it("leaves leagueTotals at zero after all members refund", async function () {
      await escrow.connect(member1).claimRefund(LEAGUE_ID);
      await escrow.connect(member2).claimRefund(LEAGUE_ID);
      expect(await escrow.leagueTotals(LEAGUE_ID)).to.equal(0);
    });
  });

  // ─── distributePayout ────────────────────────────────────────────────────────

  describe("distributePayout", function () {
    beforeEach(async function () {
      await usdc.connect(member1).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(50));
      await usdc.connect(member2).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member2).payEntryFee(LEAGUE_ID, USDC(50));
    });

    it("sends the right amounts to each recipient", async function () {
      const b1 = await usdc.balanceOf(member1.address);
      const b2 = await usdc.balanceOf(member2.address);

      await escrow.connect(owner).distributePayout(
        LEAGUE_ID,
        [member1.address, member2.address],
        [USDC(70), USDC(30)]
      );

      expect(await usdc.balanceOf(member1.address)).to.equal(b1 + USDC(70));
      expect(await usdc.balanceOf(member2.address)).to.equal(b2 + USDC(30));
    });

    it("decrements leagueTotals by the distributed amount", async function () {
      await escrow.connect(owner).distributePayout(
        LEAGUE_ID,
        [member1.address],
        [USDC(50)]
      );
      expect(await escrow.leagueTotals(LEAGUE_ID)).to.equal(USDC(50));
    });

    it("emits PayoutDistributed for each recipient", async function () {
      await expect(
        escrow.connect(owner).distributePayout(
          LEAGUE_ID,
          [member1.address, member2.address],
          [USDC(70), USDC(30)]
        )
      )
        .to.emit(escrow, "PayoutDistributed").withArgs(LEAGUE_ID, member1.address, USDC(70))
        .and.to.emit(escrow, "PayoutDistributed").withArgs(LEAGUE_ID, member2.address, USDC(30));
    });

    it("reverts when total payout exceeds collected amount", async function () {
      await expect(
        escrow.connect(owner).distributePayout(
          LEAGUE_ID,
          [member1.address],
          [USDC(101)]
        )
      ).to.be.revertedWith("payout exceeds league escrow");
    });

    it("reverts when called by non-owner", async function () {
      await expect(
        escrow.connect(member1).distributePayout(
          LEAGUE_ID,
          [member2.address],
          [USDC(50)]
        )
      ).to.be.revertedWith("not owner");
    });

    it("reverts on length mismatch between recipients and amounts", async function () {
      await expect(
        escrow.connect(owner).distributePayout(
          LEAGUE_ID,
          [member1.address, member2.address],
          [USDC(100)]
        )
      ).to.be.revertedWith("length mismatch");
    });
  });

  // ─── interaction: refund after partial payout ─────────────────────────────────

  describe("refund and payout interaction", function () {
    it("a member refund reduces the pool available for payout", async function () {
      await usdc.connect(member1).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member1).payEntryFee(LEAGUE_ID, USDC(50));
      await usdc.connect(member2).approve(await escrow.getAddress(), USDC(50));
      await escrow.connect(member2).payEntryFee(LEAGUE_ID, USDC(50));

      // member1 refunds → only $50 left in escrow
      await escrow.connect(member1).claimRefund(LEAGUE_ID);

      // Paying out $51 should now fail
      await expect(
        escrow.connect(owner).distributePayout(
          LEAGUE_ID,
          [member2.address],
          [USDC(51)]
        )
      ).to.be.revertedWith("payout exceeds league escrow");

      // Paying out exactly $50 should succeed
      await escrow.connect(owner).distributePayout(
        LEAGUE_ID,
        [member2.address],
        [USDC(50)]
      );
    });
  });
});

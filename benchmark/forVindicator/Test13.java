package test;

public class Test13 extends Thread {
  static int x;
  static int y;
  static int z;
  static final Object m = new Object();
  static final Object n = new Object();
  static final Object o = new Object();
  static final Object p = new Object();
  static final Object q = new Object();

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1
	synchronized(m) {		
		System.out.println("l(m)");
		x = 1;
		System.out.println("w(x)");
		sleepSec(10);
		int tmp = x;
		System.out.println("r(x)");
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			int tmp = x;
			System.out.println("r(x)");
			y = 1;
			System.out.println("w(y)");
			x = 2;
			System.out.println("w(x)");
		}
   }
   
   public static class T3 extends Thread implements Runnable {
		public void run() {
			sleepSec(25);
			int tmp = z;
			System.out.println("r(z)");
			y = 2;
			System.out.println("w(y)");
			z = 2;
			System.out.println("w(z)");
		}
   }
   
   public static class T4 extends Thread implements Runnable {
		public void run() {
			sleepSec(20);
			synchronized(m) {
				System.out.println("l(m)");
				z = 1;
				System.out.println("w(z)");
				sleepSec(10);
				int tmp = z;
				System.out.println("r(z)");
				System.out.println("u(m)");
			}
		}
   }


   public static void main(String args[]) throws Exception {
	final Test13 thread1 = new Test13();
    final T2 thread2 = new T2();
	final T3 thread3 = new T3();
	final T4 thread4 = new T4();
	

	thread1.start();
	thread2.start();
	thread3.start();
	thread4.start();
	

	thread1.join();
	thread2.join();
	thread3.join();
	thread4.join();

   }
}
